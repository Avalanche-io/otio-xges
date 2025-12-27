// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package xges

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/mrjoshuak/gotio/opentime"
	"github.com/mrjoshuak/gotio/opentimelineio"
)

// Decoder reads and decodes XGES data
type Decoder struct {
	r    io.Reader
	rate float64
}

// NewDecoder creates a new XGES decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:    r,
		rate: 25.0, // default frame rate
	}
}

// Decode reads XGES XML and converts it to an OTIO Timeline
func (d *Decoder) Decode() (*opentimelineio.Timeline, error) {
	var ges GES
	decoder := xml.NewDecoder(d.r)
	if err := decoder.Decode(&ges); err != nil {
		return nil, fmt.Errorf("failed to decode XGES XML: %w", err)
	}

	// Extract frame rate from video track
	d.extractFrameRate(&ges.Project.Timeline)

	// Convert to OTIO timeline
	timeline, err := d.convertTimeline(&ges.Project.Timeline)
	if err != nil {
		return nil, err
	}

	// Extract timeline name from project metadata
	if name := d.extractName(ges.Project.Metadatas); name != "" {
		timeline.SetName(name)
	}

	return timeline, nil
}

// extractFrameRate extracts the frame rate from the video track
func (d *Decoder) extractFrameRate(timeline *Timeline) {
	for _, track := range timeline.Tracks {
		if track.TrackType == TrackTypeVideo {
			if rate := d.extractFrameRateFromProperties(track.Properties); rate > 0 {
				d.rate = rate
				return
			}
		}
	}
}

// extractFrameRateFromProperties parses framerate from restriction-caps
func (d *Decoder) extractFrameRateFromProperties(props string) float64 {
	// Look for restriction-caps=(string)"..."
	re := regexp.MustCompile(`restriction-caps=\(string\)(?:"([^"]+)"|([^\s,;]+))`)
	matches := re.FindStringSubmatch(props)
	if len(matches) < 2 {
		return 0
	}

	caps := matches[1]
	if caps == "" && len(matches) > 2 {
		caps = matches[2]
	}

	// Look for framerate=(fraction)25/1 or framerate=\(fraction\)25/1
	// In the caps string, backslashes are literal characters escaping GStreamer structure syntax
	re = regexp.MustCompile(`framerate\\?=\\?\(fraction\\?\)(\d+)/(\d+)`)
	matches = re.FindStringSubmatch(caps)
	if len(matches) < 3 {
		return 0
	}

	num, err1 := strconv.ParseFloat(matches[1], 64)
	den, err2 := strconv.ParseFloat(matches[2], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}

	return num / den
}

// extractName extracts the name from a GstStructure metadata string
func (d *Decoder) extractName(metadatas string) string {
	// Look for name=(string)"value" or name=(string)value
	re := regexp.MustCompile(`name=\(string\)(?:"([^"]+)"|(\S+))`)
	matches := re.FindStringSubmatch(metadatas)
	if len(matches) > 1 {
		if matches[1] != "" {
			return d.unescapeGstString(matches[1])
		}
		if matches[2] != "" {
			return d.unescapeGstString(matches[2])
		}
	}
	return ""
}

// unescapeGstString unescapes GStreamer structure strings
func (d *Decoder) unescapeGstString(s string) string {
	s = strings.ReplaceAll(s, `\ `, ` `)
	s = strings.ReplaceAll(s, `\,`, `,`)
	s = strings.ReplaceAll(s, `\"`, `"`)
	return s
}

// convertTimeline converts an XGES Timeline to an OTIO Timeline
func (d *Decoder) convertTimeline(xgesTimeline *Timeline) (*opentimelineio.Timeline, error) {
	// Create the timeline
	timeline := opentimelineio.NewTimeline("", nil, nil)

	// Create tracks based on the track types
	tracksByType := make(map[int]*opentimelineio.Track)
	for _, track := range xgesTimeline.Tracks {
		otioTrack := d.createTrack(&track)
		tracksByType[track.TrackType] = otioTrack
	}

	// Process layers in order
	for _, layer := range xgesTimeline.Layers {
		if err := d.processLayer(&layer, tracksByType); err != nil {
			return nil, err
		}
	}

	// Add tracks to timeline
	tracks := timeline.Tracks()
	for _, trackType := range []int{TrackTypeVideo, TrackTypeAudio} {
		if track, ok := tracksByType[trackType]; ok {
			if err := tracks.AppendChild(track); err != nil {
				return nil, err
			}
		}
	}

	return timeline, nil
}

// createTrack creates an OTIO track from an XGES track
func (d *Decoder) createTrack(xgesTrack *Track) *opentimelineio.Track {
	kind := opentimelineio.TrackKindVideo
	if xgesTrack.TrackType == TrackTypeAudio {
		kind = opentimelineio.TrackKindAudio
	}

	return opentimelineio.NewTrack("", nil, kind, nil, nil)
}

// processLayer processes an XGES layer and adds clips to tracks
func (d *Decoder) processLayer(layer *Layer, tracksByType map[int]*opentimelineio.Track) error {
	// Group clips by track type and sort by start time
	clipsByTrack := make(map[int][]Clip)
	for _, clip := range layer.Clips {
		// Determine which tracks this clip belongs to (based on track-types bitmask)
		for trackType := range tracksByType {
			if clip.TrackTypes&trackType != 0 {
				clipsByTrack[trackType] = append(clipsByTrack[trackType], clip)
			}
		}
	}

	// Process clips for each track
	for trackType, clips := range clipsByTrack {
		track := tracksByType[trackType]
		if err := d.addClipsToTrack(track, clips); err != nil {
			return err
		}
	}

	return nil
}

// addClipsToTrack adds clips to an OTIO track, filling gaps as needed
func (d *Decoder) addClipsToTrack(track *opentimelineio.Track, clips []Clip) error {
	if len(clips) == 0 {
		return nil
	}

	// Sort clips by start time
	// (already sorted in layer, but let's be safe)

	var currentTime uint64 = 0

	for _, xgesClip := range clips {
		// Add gap if needed
		if xgesClip.Start > currentTime {
			gapDuration := d.toRationalTime(xgesClip.Start - currentTime)
			gap := opentimelineio.NewGapWithDuration(gapDuration)
			if err := track.AppendChild(gap); err != nil {
				return err
			}
		}

		// Convert and add the clip
		otioItem, err := d.convertClip(&xgesClip)
		if err != nil {
			return err
		}

		if otioItem != nil {
			if err := track.AppendChild(otioItem); err != nil {
				return err
			}
		}

		currentTime = xgesClip.Start + xgesClip.Duration
	}

	return nil
}

// convertClip converts an XGES clip to an OTIO composable
func (d *Decoder) convertClip(xgesClip *Clip) (opentimelineio.Composable, error) {
	// Handle transition clips
	if xgesClip.TypeName == ClipTypeTransition {
		return d.convertTransition(xgesClip), nil
	}

	// Handle URI clips
	if xgesClip.TypeName == ClipTypeURI {
		return d.convertURIClip(xgesClip), nil
	}

	// Handle test clips (generators/test patterns)
	if xgesClip.TypeName == ClipTypeTest {
		return d.convertTestClip(xgesClip), nil
	}

	// Handle title clips (text overlays)
	if xgesClip.TypeName == ClipTypeTitle {
		return d.convertTitleClip(xgesClip), nil
	}

	// Unsupported clip type - return a gap
	duration := d.toRationalTime(xgesClip.Duration)
	return opentimelineio.NewGapWithDuration(duration), nil
}

// convertTransition converts a transition clip to an OTIO Transition
func (d *Decoder) convertTransition(xgesClip *Clip) opentimelineio.Composable {
	duration := d.toRationalTime(xgesClip.Duration)

	// Map GES transition types to OTIO transition types
	transitionType := d.mapTransitionType(xgesClip.AssetID)

	name := d.extractName(xgesClip.Properties)

	// Create transition with in and out offsets (half duration each)
	halfDuration := opentime.NewRationalTime(duration.Value()/2, duration.Rate())

	transition := opentimelineio.NewTransition(
		name,
		opentimelineio.TransitionType(transitionType),
		halfDuration,
		halfDuration,
		nil,
	)

	// Store children-properties as metadata (for transition parameters)
	if xgesClip.ChildrenProperties != "" {
		if transition.Metadata() == nil {
			transition.SetMetadata(make(map[string]interface{}))
		}
		transition.Metadata()["xges"] = map[string]interface{}{
			"children-properties": xgesClip.ChildrenProperties,
		}
	}

	return transition
}

// mapTransitionType maps GES transition asset IDs to OTIO transition types
func (d *Decoder) mapTransitionType(assetID string) string {
	// Common GES transition types
	switch strings.ToLower(assetID) {
	case "crossfade":
		return string(opentimelineio.TransitionTypeSMPTEDissolve)
	case "wipe":
		return "SMPTE_Wipe"
	case "clock-wipe":
		return "SMPTE_ClockWipe"
	case "barwipe":
		return "SMPTE_BarWipe"
	case "box-wipe":
		return "SMPTE_BoxWipe"
	default:
		// Store original asset-id in custom type
		return assetID
	}
}

// convertURIClip converts a URI clip to an OTIO Clip
func (d *Decoder) convertURIClip(xgesClip *Clip) *opentimelineio.Clip {
	name := d.extractName(xgesClip.Properties)

	// Create source range
	start := d.toRationalTime(xgesClip.Inpoint)
	duration := d.toRationalTime(xgesClip.Duration)
	sourceRange := opentime.NewTimeRange(start, duration)

	// Create media reference
	mediaRef := opentimelineio.NewExternalReference(
		"",              // name
		xgesClip.AssetID, // target URL
		nil,             // available range - could be extracted from asset
		nil,             // metadata
	)

	// Create clip
	clip := opentimelineio.NewClip(
		name,
		mediaRef,
		&sourceRange,
		nil,
		nil,
		nil,
		"",
		nil,
	)

	// Store children-properties as metadata if present
	d.addChildrenPropertiesToMetadata(clip, xgesClip)

	return clip
}

// convertTestClip converts a GESTestClip to an OTIO Clip with GeneratorReference
func (d *Decoder) convertTestClip(xgesClip *Clip) *opentimelineio.Clip {
	name := d.extractName(xgesClip.Properties)

	// Create source range
	start := d.toRationalTime(xgesClip.Inpoint)
	duration := d.toRationalTime(xgesClip.Duration)
	sourceRange := opentime.NewTimeRange(start, duration)

	// Create generator reference
	// asset-id typically contains the test pattern type (e.g., "bars", "snow", "black")
	generatorKind := xgesClip.AssetID
	if generatorKind == "" {
		generatorKind = "black"
	}

	mediaRef := opentimelineio.NewGeneratorReference(
		"",            // name
		generatorKind, // generator kind
		nil,           // parameters
		nil,           // available range
		nil,           // metadata
	)

	// Create clip
	clip := opentimelineio.NewClip(
		name,
		mediaRef,
		&sourceRange,
		nil,
		nil,
		nil,
		"",
		nil,
	)

	// Store children-properties as metadata if present
	d.addChildrenPropertiesToMetadata(clip, xgesClip)

	return clip
}

// convertTitleClip converts a GESTitleClip to an OTIO Clip with metadata
func (d *Decoder) convertTitleClip(xgesClip *Clip) *opentimelineio.Clip {
	name := d.extractName(xgesClip.Properties)

	// Create source range
	start := d.toRationalTime(xgesClip.Inpoint)
	duration := d.toRationalTime(xgesClip.Duration)
	sourceRange := opentime.NewTimeRange(start, duration)

	// Create a generator reference for title/text overlay
	mediaRef := opentimelineio.NewGeneratorReference(
		"",      // name
		"title", // generator kind
		nil,     // parameters
		nil,     // available range
		nil,     // metadata
	)

	// Create clip
	clip := opentimelineio.NewClip(
		name,
		mediaRef,
		&sourceRange,
		nil,
		nil,
		nil,
		"",
		nil,
	)

	// Store title text and properties in metadata
	metadata := make(map[string]interface{})
	xgesMetadata := make(map[string]interface{})

	// Extract text from children-properties
	if xgesClip.ChildrenProperties != "" {
		xgesMetadata["children-properties"] = xgesClip.ChildrenProperties
		// Try to extract text property
		textMatch := regexp.MustCompile(`text=\(string\)(?:"([^"]+)"|(\S+))`).FindStringSubmatch(xgesClip.ChildrenProperties)
		if len(textMatch) > 1 {
			text := textMatch[1]
			if text == "" && len(textMatch) > 2 {
				text = textMatch[2]
			}
			xgesMetadata["text"] = d.unescapeGstString(text)
		}
	}

	// Store clip type
	xgesMetadata["clip-type"] = "title"

	metadata["xges"] = xgesMetadata
	clip.SetMetadata(metadata)

	return clip
}

// addChildrenPropertiesToMetadata adds children-properties to clip metadata
func (d *Decoder) addChildrenPropertiesToMetadata(clip *opentimelineio.Clip, xgesClip *Clip) {
	if xgesClip.ChildrenProperties == "" {
		return
	}

	metadata := clip.Metadata()
	if metadata == nil {
		metadata = make(map[string]interface{})
		clip.SetMetadata(metadata)
	}

	xgesMetadata, ok := metadata["xges"].(map[string]interface{})
	if !ok {
		xgesMetadata = make(map[string]interface{})
		metadata["xges"] = xgesMetadata
	}

	xgesMetadata["children-properties"] = xgesClip.ChildrenProperties
}

// toRationalTime converts nanoseconds to RationalTime
func (d *Decoder) toRationalTime(ns uint64) opentime.RationalTime {
	seconds := float64(ns) / float64(GSTSecond)
	return opentime.FromSeconds(seconds, d.rate)
}

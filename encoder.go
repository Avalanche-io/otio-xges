// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package xges

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio"
)

// Encoder writes OTIO timelines as XGES XML
type Encoder struct {
	w    io.Writer
	rate float64
}

// NewEncoder creates a new XGES encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:    w,
		rate: 25.0, // default frame rate
	}
}

// Encode converts an OTIO Timeline to XGES and writes it
func (e *Encoder) Encode(timeline *gotio.Timeline) error {
	// Determine the frame rate from the timeline
	e.extractFrameRate(timeline)

	// Create GES structure
	ges := &GES{
		Version: "0.3",
		Project: Project{
			Properties: "properties;",
			Metadatas:  e.buildProjectMetadatas(timeline),
			Timeline: Timeline{
				Properties: e.buildTimelineProperties(),
				Metadatas:  e.buildTimelineMetadatas(timeline),
			},
		},
	}

	// Add tracks
	trackID := 0
	videoTracks := timeline.VideoTracks()
	audioTracks := timeline.AudioTracks()

	if len(videoTracks) > 0 {
		ges.Project.Timeline.Tracks = append(ges.Project.Timeline.Tracks, Track{
			Caps:       "video/x-raw(ANY)",
			TrackType:  TrackTypeVideo,
			TrackID:    trackID,
			Properties: e.buildVideoTrackProperties(),
			Metadatas:  "metadatas;",
		})
		trackID++
	}

	if len(audioTracks) > 0 {
		ges.Project.Timeline.Tracks = append(ges.Project.Timeline.Tracks, Track{
			Caps:       "audio/x-raw(ANY)",
			TrackType:  TrackTypeAudio,
			TrackID:    trackID,
			Properties: e.buildAudioTrackProperties(),
			Metadatas:  "metadatas;",
		})
		trackID++
	}

	// Convert tracks to layers
	clipID := 0
	layerPriority := 0

	// Process video tracks
	for _, track := range videoTracks {
		layer, err := e.convertTrackToLayer(track, layerPriority, &clipID, TrackTypeVideo)
		if err != nil {
			return err
		}
		ges.Project.Timeline.Layers = append(ges.Project.Timeline.Layers, *layer)
		layerPriority++
	}

	// Process audio tracks
	for _, track := range audioTracks {
		layer, err := e.convertTrackToLayer(track, layerPriority, &clipID, TrackTypeAudio)
		if err != nil {
			return err
		}
		ges.Project.Timeline.Layers = append(ges.Project.Timeline.Layers, *layer)
		layerPriority++
	}

	// Write XML with proper formatting
	output, err := xml.MarshalIndent(ges, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal XGES: %w", err)
	}

	// Write XML declaration
	if _, err := e.w.Write([]byte(xml.Header)); err != nil {
		return err
	}

	// Write the XML content
	if _, err := e.w.Write(output); err != nil {
		return err
	}

	// Write final newline
	if _, err := e.w.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}

// extractFrameRate extracts the frame rate from the timeline
func (e *Encoder) extractFrameRate(timeline *gotio.Timeline) {
	// Try to get rate from first video clip
	for _, track := range timeline.VideoTracks() {
		for _, child := range track.Children() {
			if clip, ok := child.(*gotio.Clip); ok {
				dur, err := clip.Duration()
				if err == nil && dur.Rate() > 0 {
					e.rate = dur.Rate()
					return
				}
			}
		}
	}

	// Try audio tracks
	for _, track := range timeline.AudioTracks() {
		for _, child := range track.Children() {
			if clip, ok := child.(*gotio.Clip); ok {
				dur, err := clip.Duration()
				if err == nil && dur.Rate() > 0 {
					e.rate = dur.Rate()
					return
				}
			}
		}
	}
}

// buildProjectMetadatas creates project metadata string
func (e *Encoder) buildProjectMetadatas(timeline *gotio.Timeline) string {
	if timeline.Name() == "" {
		return "metadatas;"
	}

	escapedName := e.escapeGstString(timeline.Name())
	return fmt.Sprintf(`metadatas, name=(string)"%s";`, escapedName)
}

// buildTimelineProperties creates timeline properties string
func (e *Encoder) buildTimelineProperties() string {
	return "properties, auto-transition=(boolean)true;"
}

// buildTimelineMetadatas creates timeline metadata string
func (e *Encoder) buildTimelineMetadatas(timeline *gotio.Timeline) string {
	parts := []string{"metadatas"}

	// Add framerate
	parts = append(parts, fmt.Sprintf("framerate=(fraction)%d/1", int(e.rate)))

	return strings.Join(parts, ", ") + ";"
}

// buildVideoTrackProperties creates video track properties
func (e *Encoder) buildVideoTrackProperties() string {
	return fmt.Sprintf(
		`properties, restriction-caps=(string)"video/x-raw\,\ width\=\(int\)1920\,\ height\=\(int\)1080\,\ framerate\=\(fraction\)%d/1", mixing=(boolean)true;`,
		int(e.rate),
	)
}

// buildAudioTrackProperties creates audio track properties
func (e *Encoder) buildAudioTrackProperties() string {
	return `properties, restriction-caps=(string)"audio/x-raw\,\ rate\=\(int\)48000\,\ channels\=\(int\)2", mixing=(boolean)true;`
}

// convertTrackToLayer converts an OTIO track to an XGES layer
func (e *Encoder) convertTrackToLayer(track *gotio.Track, priority int, clipID *int, trackType int) (*Layer, error) {
	layer := &Layer{
		Priority:   priority,
		Properties: "properties, auto-transition=(boolean)true;",
		Metadatas:  "metadatas, volume=(float)1;",
		Clips:      []Clip{},
	}

	var currentTime uint64 = 0

	for _, child := range track.Children() {
		// Skip gaps - they're implicit in XGES
		if _, isGap := child.(*gotio.Gap); isGap {
			dur, err := child.Duration()
			if err != nil {
				return nil, err
			}
			currentTime += e.toNanoseconds(dur)
			continue
		}

		// Convert clip
		if clip, isClip := child.(*gotio.Clip); isClip {
			xgesClip, err := e.convertClip(clip, currentTime, priority, trackType, *clipID)
			if err != nil {
				return nil, err
			}
			layer.Clips = append(layer.Clips, *xgesClip)
			*clipID++

			dur, err := clip.Duration()
			if err != nil {
				return nil, err
			}
			currentTime += e.toNanoseconds(dur)
			continue
		}

		// Convert transition
		if transition, isTrans := child.(*gotio.Transition); isTrans {
			xgesClip, err := e.convertTransition(transition, currentTime, priority, trackType, *clipID)
			if err != nil {
				return nil, err
			}
			layer.Clips = append(layer.Clips, *xgesClip)
			*clipID++

			// Transitions overlap, so adjust time
			dur := transition.InOffset().Add(transition.OutOffset())
			currentTime += e.toNanoseconds(dur)
			continue
		}
	}

	return layer, nil
}

// convertClip converts an OTIO Clip to an XGES Clip
func (e *Encoder) convertClip(clip *gotio.Clip, startTime uint64, priority int, trackType int, id int) (*Clip, error) {
	duration, err := clip.Duration()
	if err != nil {
		return nil, err
	}

	// Get source range
	var inpoint uint64 = 0
	if clip.SourceRange() != nil {
		inpoint = e.toNanoseconds(clip.SourceRange().StartTime())
	}

	name := clip.Name()
	if name == "" {
		name = fmt.Sprintf("clip%d", id)
	}

	// Determine clip type and asset ID based on media reference
	var assetID, typeName string
	var childrenProps string

	if ref := clip.MediaReference(); ref != nil {
		switch mediaRef := ref.(type) {
		case *gotio.ExternalReference:
			// URI clip
			assetID = mediaRef.TargetURL()
			typeName = ClipTypeURI
			if assetID == "" {
				assetID = "file:///missing"
			}

		case *gotio.GeneratorReference:
			// Check if this is a title clip
			if mediaRef.GeneratorKind() == "title" {
				typeName = ClipTypeTitle
				assetID = ClipTypeTitle
				childrenProps = e.extractTitleProperties(clip)
			} else {
				// Test pattern/generator clip
				typeName = ClipTypeTest
				assetID = mediaRef.GeneratorKind()
				if assetID == "" {
					assetID = "black"
				}
			}

		default:
			// Fallback to URI clip
			assetID = "file:///missing"
			typeName = ClipTypeURI
		}
	} else {
		// No media reference - default to URI clip
		assetID = "file:///missing"
		typeName = ClipTypeURI
	}

	// Extract children-properties from metadata if present
	if childrenProps == "" {
		childrenProps = e.extractChildrenProperties(clip)
	}

	xgesClip := &Clip{
		ID:            id,
		AssetID:       assetID,
		TypeName:      typeName,
		LayerPriority: priority,
		TrackTypes:    trackType,
		Start:         startTime,
		Duration:      e.toNanoseconds(duration),
		Inpoint:       inpoint,
		Rate:          0,
		Properties:    e.buildClipProperties(name),
	}

	if childrenProps != "" {
		xgesClip.ChildrenProperties = childrenProps
	}

	return xgesClip, nil
}

// extractTitleProperties extracts title text and builds children-properties
func (e *Encoder) extractTitleProperties(clip *gotio.Clip) string {
	metadata := clip.Metadata()
	if metadata == nil {
		return ""
	}

	xgesMetadata, ok := metadata["xges"].(map[string]interface{})
	if !ok {
		return ""
	}

	// Check for existing children-properties
	if childProps, ok := xgesMetadata["children-properties"].(string); ok {
		return childProps
	}

	// Build from text metadata
	if text, ok := xgesMetadata["text"].(string); ok {
		escapedText := e.escapeGstString(text)
		return fmt.Sprintf(`properties, text=(string)"%s";`, escapedText)
	}

	return ""
}

// extractChildrenProperties extracts children-properties from clip metadata
func (e *Encoder) extractChildrenProperties(clip *gotio.Clip) string {
	metadata := clip.Metadata()
	if metadata == nil {
		return ""
	}

	xgesMetadata, ok := metadata["xges"].(map[string]interface{})
	if !ok {
		return ""
	}

	if childProps, ok := xgesMetadata["children-properties"].(string); ok {
		return childProps
	}

	return ""
}

// convertTransition converts an OTIO Transition to an XGES Clip
func (e *Encoder) convertTransition(transition *gotio.Transition, startTime uint64, priority int, trackType int, id int) (*Clip, error) {
	duration := transition.InOffset().Add(transition.OutOffset())

	// Map OTIO transition type to GES asset ID
	assetID := e.reverseMapTransitionType(transition.TransitionType())

	name := transition.Name()
	if name == "" {
		name = fmt.Sprintf("transition%d", id)
	}

	// Extract children-properties from metadata if present
	childrenProps := ""
	metadata := transition.Metadata()
	if metadata != nil {
		if xgesMetadata, ok := metadata["xges"].(map[string]interface{}); ok {
			if childProps, ok := xgesMetadata["children-properties"].(string); ok {
				childrenProps = childProps
			}
		}
	}

	xgesClip := &Clip{
		ID:            id,
		AssetID:       assetID,
		TypeName:      ClipTypeTransition,
		LayerPriority: priority,
		TrackTypes:    trackType,
		Start:         startTime,
		Duration:      e.toNanoseconds(duration),
		Inpoint:       0,
		Rate:          0,
		Properties:    e.buildClipProperties(name),
	}

	if childrenProps != "" {
		xgesClip.ChildrenProperties = childrenProps
	}

	return xgesClip, nil
}

// reverseMapTransitionType maps OTIO transition types back to GES asset IDs
func (e *Encoder) reverseMapTransitionType(transitionType gotio.TransitionType) string {
	switch transitionType {
	case gotio.TransitionTypeSMPTEDissolve:
		return "crossfade"
	case "SMPTE_Wipe":
		return "wipe"
	case "SMPTE_ClockWipe":
		return "clock-wipe"
	case "SMPTE_BarWipe":
		return "barwipe"
	case "SMPTE_BoxWipe":
		return "box-wipe"
	default:
		// For custom transitions, use the type as-is
		if transitionType == "" || transitionType == gotio.TransitionTypeCustom {
			return "crossfade"
		}
		return string(transitionType)
	}
}

// buildClipProperties creates clip properties string
func (e *Encoder) buildClipProperties(name string) string {
	escapedName := e.escapeGstString(name)
	return fmt.Sprintf(`properties, name=(string)"%s", mute=(boolean)false, is-image=(boolean)false;`, escapedName)
}

// escapeGstString escapes strings for GStreamer structures
func (e *Encoder) escapeGstString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, ` `, `\ `)
	s = strings.ReplaceAll(s, `,`, `\,`)
	return s
}

// toNanoseconds converts RationalTime to nanoseconds
func (e *Encoder) toNanoseconds(t opentime.RationalTime) uint64 {
	seconds := t.ToSeconds()
	return uint64(seconds * float64(GSTSecond))
}

// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package xges

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mrjoshuak/gotio/opentime"
	"github.com/mrjoshuak/gotio/opentimelineio"
)

const simpleXGES = `<?xml version="1.0" ?>
<ges version='0.3'>
  <project properties='properties;' metadatas='metadatas, name=(string)"Test\ Project";'>
    <timeline properties='properties, auto-transition=(boolean)true;' metadatas='metadatas, framerate=(fraction)25/1;'>
      <track caps='video/x-raw(ANY)' track-type='4' track-id='0' properties='properties, restriction-caps=(string)"video/x-raw\,\ width\=\(int\)1920\,\ height\=\(int\)1080\,\ framerate\=\(fraction\)25/1", mixing=(boolean)true;' metadatas='metadatas;'/>
      <track caps='audio/x-raw(ANY)' track-type='2' track-id='1' properties='properties, restriction-caps=(string)"audio/x-raw\,\ rate\=\(int\)48000\,\ channels\=\(int\)2", mixing=(boolean)true;' metadatas='metadatas;'/>
      <layer priority='0' properties='properties, auto-transition=(boolean)true;' metadatas='metadatas, volume=(float)1;'>
        <clip id='0' asset-id='file:///example/video.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='0' duration='1000000000' inpoint='0' rate='0' properties='properties, name=(string)"clip1", mute=(boolean)false, is-image=(boolean)false;' />
        <clip id='1' asset-id='file:///example/video2.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='1000000000' duration='2000000000' inpoint='500000000' rate='0' properties='properties, name=(string)"clip2", mute=(boolean)false, is-image=(boolean)false;' />
      </layer>
      <layer priority='1' properties='properties, auto-transition=(boolean)true;' metadatas='metadatas, volume=(float)1;'>
        <clip id='2' asset-id='file:///example/audio.wav' type-name='GESUriClip' layer-priority='1' track-types='2' start='0' duration='3000000000' inpoint='0' rate='0' properties='properties, name=(string)"audio1", mute=(boolean)false, is-image=(boolean)false;' />
      </layer>
    </timeline>
  </project>
</ges>
`

const transitionXGES = `<?xml version="1.0" ?>
<ges version='0.3'>
  <project properties='properties;'>
    <timeline properties='properties;' metadatas='metadatas, framerate=(fraction)30/1;'>
      <track caps='video/x-raw(ANY)' track-type='4' track-id='0' properties='properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)30/1";'/>
      <layer priority='0'>
        <clip id='0' asset-id='file:///clip1.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='0' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"clip1";' />
        <clip id='1' asset-id='crossfade' type-name='GESTransitionClip' layer-priority='0' track-types='4' start='1500000000' duration='1000000000' inpoint='0' rate='0' properties='properties, name=(string)"transition1";' />
        <clip id='2' asset-id='file:///clip2.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='1500000000' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"clip2";' />
      </layer>
    </timeline>
  </project>
</ges>
`

func TestDecoder_DecodeSimple(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(simpleXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if timeline == nil {
		t.Fatal("Timeline is nil")
	}

	// Check timeline name
	if timeline.Name() != "Test Project" {
		t.Errorf("Expected timeline name 'Test Project', got '%s'", timeline.Name())
	}

	// Check tracks
	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Errorf("Expected 1 video track, got %d", len(videoTracks))
	}

	audioTracks := timeline.AudioTracks()
	if len(audioTracks) != 1 {
		t.Errorf("Expected 1 audio track, got %d", len(audioTracks))
	}

	// Check video track clips
	if len(videoTracks) > 0 {
		videoTrack := videoTracks[0]
		children := videoTrack.Children()

		// Should have 2 clips
		clipCount := 0
		for _, child := range children {
			if _, ok := child.(*opentimelineio.Clip); ok {
				clipCount++
			}
		}

		if clipCount != 2 {
			t.Errorf("Expected 2 clips in video track, got %d", clipCount)
		}

		// Check first clip
		if clip, ok := children[0].(*opentimelineio.Clip); ok {
			if clip.Name() != "clip1" {
				t.Errorf("Expected first clip name 'clip1', got '%s'", clip.Name())
			}

			dur, _ := clip.Duration()
			expectedDur := opentime.FromSeconds(1.0, 25.0)
			if dur.Value() != expectedDur.Value() {
				t.Errorf("Expected first clip duration %v, got %v", expectedDur, dur)
			}

			if ref := clip.MediaReference(); ref != nil {
				if extRef, ok := ref.(*opentimelineio.ExternalReference); ok {
					if extRef.TargetURL() != "file:///example/video.mp4" {
						t.Errorf("Expected URL 'file:///example/video.mp4', got '%s'", extRef.TargetURL())
					}
				}
			}
		}

		// Check second clip
		if len(children) > 1 {
			if clip, ok := children[1].(*opentimelineio.Clip); ok {
				if clip.Name() != "clip2" {
					t.Errorf("Expected second clip name 'clip2', got '%s'", clip.Name())
				}

				dur, _ := clip.Duration()
				expectedDur := opentime.FromSeconds(2.0, 25.0)
				if dur.Value() != expectedDur.Value() {
					t.Errorf("Expected second clip duration %v, got %v", expectedDur, dur)
				}

				// Check inpoint
				if clip.SourceRange() != nil {
					start := clip.SourceRange().StartTime()
					expectedStart := opentime.FromSeconds(0.5, 25.0)
					if start.Value() != expectedStart.Value() {
						t.Errorf("Expected inpoint %v, got %v", expectedStart, start)
					}
				}
			}
		}
	}

	// Check audio track
	if len(audioTracks) > 0 {
		audioTrack := audioTracks[0]
		children := audioTrack.Children()

		clipCount := 0
		for _, child := range children {
			if _, ok := child.(*opentimelineio.Clip); ok {
				clipCount++
			}
		}

		if clipCount != 1 {
			t.Errorf("Expected 1 clip in audio track, got %d", clipCount)
		}
	}
}

func TestDecoder_DecodeWithTransition(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(transitionXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Check frame rate extraction
	if decoder.rate != 30.0 {
		t.Errorf("Expected rate 30.0, got %f", decoder.rate)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	children := videoTracks[0].Children()

	// Count transitions and clips
	transitionCount := 0
	clipCount := 0

	for _, child := range children {
		switch child.(type) {
		case *opentimelineio.Transition:
			transitionCount++
		case *opentimelineio.Clip:
			clipCount++
		}
	}

	if clipCount < 2 {
		t.Errorf("Expected at least 2 clips, got %d", clipCount)
	}

	if transitionCount < 1 {
		t.Errorf("Expected at least 1 transition, got %d", transitionCount)
	}
}

func TestEncoder_Encode(t *testing.T) {
	// Create a simple timeline
	timeline := opentimelineio.NewTimeline("My Timeline", nil, nil)

	// Create video track
	videoTrack := opentimelineio.NewTrack("video", nil, opentimelineio.TrackKindVideo, nil, nil)

	// Add clips
	rate := 25.0
	ref1 := opentimelineio.NewExternalReference("", "file:///test1.mp4", nil, nil)
	sourceRange1 := opentime.NewTimeRange(
		opentime.NewRationalTime(0, rate),
		opentime.NewRationalTime(50, rate), // 2 seconds at 25fps
	)
	clip1 := opentimelineio.NewClip("clip1", ref1, &sourceRange1, nil, nil, nil, "", nil)

	ref2 := opentimelineio.NewExternalReference("", "file:///test2.mp4", nil, nil)
	sourceRange2 := opentime.NewTimeRange(
		opentime.NewRationalTime(25, rate),
		opentime.NewRationalTime(75, rate), // 3 seconds at 25fps
	)
	clip2 := opentimelineio.NewClip("clip2", ref2, &sourceRange2, nil, nil, nil, "", nil)

	videoTrack.AppendChild(clip1)
	videoTrack.AppendChild(clip2)

	timeline.Tracks().AppendChild(videoTrack)

	// Encode
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "<ges version") {
		t.Error("Output missing <ges> element")
	}

	if !strings.Contains(output, "<project") {
		t.Error("Output missing <project> element")
	}

	if !strings.Contains(output, "<timeline") {
		t.Error("Output missing <timeline> element")
	}

	if !strings.Contains(output, "track-type=\"4\"") {
		t.Error("Output missing video track")
	}

	if !strings.Contains(output, "file:///test1.mp4") {
		t.Error("Output missing first clip reference")
	}

	if !strings.Contains(output, "file:///test2.mp4") {
		t.Error("Output missing second clip reference")
	}

	if !strings.Contains(output, "My\\ Timeline") {
		t.Error("Output missing escaped timeline name")
	}
}

func TestRoundTrip(t *testing.T) {
	// Decode the simple XGES
	decoder := NewDecoder(strings.NewReader(simpleXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Encode it back
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err = encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode again
	decoder2 := NewDecoder(&buf)
	timeline2, err := decoder2.Decode()
	if err != nil {
		t.Fatalf("Second decode failed: %v", err)
	}

	// Compare basic properties
	if timeline.Name() != timeline2.Name() {
		t.Errorf("Timeline names differ: '%s' vs '%s'", timeline.Name(), timeline2.Name())
	}

	videoTracks1 := timeline.VideoTracks()
	videoTracks2 := timeline2.VideoTracks()

	if len(videoTracks1) != len(videoTracks2) {
		t.Errorf("Video track counts differ: %d vs %d", len(videoTracks1), len(videoTracks2))
	}

	audioTracks1 := timeline.AudioTracks()
	audioTracks2 := timeline2.AudioTracks()

	if len(audioTracks1) != len(audioTracks2) {
		t.Errorf("Audio track counts differ: %d vs %d", len(audioTracks1), len(audioTracks2))
	}
}

func TestExtractFrameRate(t *testing.T) {
	testCases := []struct {
		name     string
		props    string
		expected float64
	}{
		{
			name:     "25fps",
			props:    `properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)25/1";`,
			expected: 25.0,
		},
		{
			name:     "30fps",
			props:    `properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)30/1";`,
			expected: 30.0,
		},
		{
			name:     "23.976fps",
			props:    `properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)24000/1001";`,
			expected: 24000.0 / 1001.0,
		},
		{
			name:     "no framerate",
			props:    `properties, restriction-caps=(string)"video/x-raw";`,
			expected: 0,
		},
		{
			name:     "from actual test data",
			props:    `properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)30/1";`,
			expected: 30.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(nil)
			rate := decoder.extractFrameRateFromProperties(tc.props)
			// Allow small floating point error
			if rate != tc.expected && (tc.expected == 0 || rate == 0) {
				t.Errorf("Expected rate %f, got %f", tc.expected, rate)
			} else if tc.expected != 0 && rate != 0 {
				diff := rate - tc.expected
				if diff < 0 {
					diff = -diff
				}
				if diff > 0.0001 {
					t.Errorf("Expected rate %f, got %f (diff %f)", tc.expected, rate, diff)
				}
			}
		})
	}
}

func TestGstStringEscaping(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", `with\ space`},
		{"with,comma", `with\,comma`},
		{`with"quote`, `with\"quote`},
		{"complex test, with spaces", `complex\ test\,\ with\ spaces`},
	}

	encoder := NewEncoder(nil)
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := encoder.escapeGstString(tc.input)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestGstStringUnescaping(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with\ space`, "with space"},
		{`with\,comma`, "with,comma"},
		{`with\"quote`, `with"quote`},
		{`complex\ test\,\ with\ spaces`, "complex test, with spaces"},
	}

	decoder := NewDecoder(nil)
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := decoder.unescapeGstString(tc.input)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

const testClipsXGES = `<?xml version="1.0" ?>
<ges version='0.3'>
  <project properties='properties;'>
    <timeline properties='properties;' metadatas='metadatas, framerate=(fraction)25/1;'>
      <track caps='video/x-raw(ANY)' track-type='4' track-id='0' properties='properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)25/1";'/>
      <layer priority='0'>
        <clip id='0' asset-id='bars' type-name='GESTestClip' layer-priority='0' track-types='4' start='0' duration='1000000000' inpoint='0' rate='0' properties='properties, name=(string)"test-pattern";' children-properties='properties, pattern=(int)0;'/>
        <clip id='1' asset-id='GESTitleClip' type-name='GESTitleClip' layer-priority='0' track-types='4' start='1000000000' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"title-overlay";' children-properties='properties, text=(string)"Hello\ World";'/>
      </layer>
    </timeline>
  </project>
</ges>
`

const multiTransitionXGES = `<?xml version="1.0" ?>
<ges version='0.3'>
  <project properties='properties;'>
    <timeline properties='properties;' metadatas='metadatas, framerate=(fraction)30/1;'>
      <track caps='video/x-raw(ANY)' track-type='4' track-id='0' properties='properties;'/>
      <layer priority='0'>
        <clip id='0' asset-id='file:///clip1.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='0' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"clip1";' />
        <clip id='1' asset-id='wipe' type-name='GESTransitionClip' layer-priority='0' track-types='4' start='1500000000' duration='1000000000' inpoint='0' rate='0' properties='properties, name=(string)"wipe-transition";' children-properties='properties, GESVideoTransition::border=(uint)0;'/>
        <clip id='2' asset-id='file:///clip2.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='1500000000' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"clip2";' />
      </layer>
    </timeline>
  </project>
</ges>
`

func TestDecoder_TestClip(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(testClipsXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	children := videoTracks[0].Children()

	// Find the test clip
	var testClip *opentimelineio.Clip
	for _, child := range children {
		if clip, ok := child.(*opentimelineio.Clip); ok {
			if clip.Name() == "test-pattern" {
				testClip = clip
				break
			}
		}
	}

	if testClip == nil {
		t.Fatal("Test pattern clip not found")
	}

	// Check it's a GeneratorReference
	ref := testClip.MediaReference()
	if ref == nil {
		t.Fatal("Test clip has no media reference")
	}

	genRef, ok := ref.(*opentimelineio.GeneratorReference)
	if !ok {
		t.Fatalf("Expected GeneratorReference, got %T", ref)
	}

	if genRef.GeneratorKind() != "bars" {
		t.Errorf("Expected generator kind 'bars', got '%s'", genRef.GeneratorKind())
	}

	// Check children-properties in metadata
	metadata := testClip.Metadata()
	if metadata != nil {
		if xges, ok := metadata["xges"].(map[string]interface{}); ok {
			if childProps, ok := xges["children-properties"].(string); ok {
				if !strings.Contains(childProps, "pattern=(int)0") {
					t.Errorf("Expected children-properties to contain pattern, got '%s'", childProps)
				}
			}
		}
	}
}

func TestDecoder_TitleClip(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(testClipsXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	children := videoTracks[0].Children()

	// Find the title clip
	var titleClip *opentimelineio.Clip
	for _, child := range children {
		if clip, ok := child.(*opentimelineio.Clip); ok {
			if clip.Name() == "title-overlay" {
				titleClip = clip
				break
			}
		}
	}

	if titleClip == nil {
		t.Fatal("Title clip not found")
	}

	// Check it's a GeneratorReference with kind "title"
	ref := titleClip.MediaReference()
	if ref == nil {
		t.Fatal("Title clip has no media reference")
	}

	genRef, ok := ref.(*opentimelineio.GeneratorReference)
	if !ok {
		t.Fatalf("Expected GeneratorReference, got %T", ref)
	}

	if genRef.GeneratorKind() != "title" {
		t.Errorf("Expected generator kind 'title', got '%s'", genRef.GeneratorKind())
	}

	// Check text in metadata
	metadata := titleClip.Metadata()
	if metadata == nil {
		t.Fatal("Title clip has no metadata")
	}

	xges, ok := metadata["xges"].(map[string]interface{})
	if !ok {
		t.Fatal("Title clip metadata missing 'xges' key")
	}

	text, ok := xges["text"].(string)
	if !ok {
		t.Fatal("Title clip metadata missing 'text' key")
	}

	if text != "Hello World" {
		t.Errorf("Expected text 'Hello World', got '%s'", text)
	}

	clipType, ok := xges["clip-type"].(string)
	if !ok || clipType != "title" {
		t.Errorf("Expected clip-type 'title', got '%v'", clipType)
	}
}

func TestDecoder_TransitionTypes(t *testing.T) {
	decoder := NewDecoder(strings.NewReader(multiTransitionXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	videoTracks := timeline.VideoTracks()
	if len(videoTracks) != 1 {
		t.Fatalf("Expected 1 video track, got %d", len(videoTracks))
	}

	children := videoTracks[0].Children()

	// Find the wipe transition
	var wipeTransition *opentimelineio.Transition
	for _, child := range children {
		if trans, ok := child.(*opentimelineio.Transition); ok {
			if trans.Name() == "wipe-transition" {
				wipeTransition = trans
				break
			}
		}
	}

	if wipeTransition == nil {
		t.Fatal("Wipe transition not found")
	}

	if wipeTransition.TransitionType() != "SMPTE_Wipe" {
		t.Errorf("Expected transition type 'SMPTE_Wipe', got '%s'", wipeTransition.TransitionType())
	}

	// Check children-properties in metadata
	metadata := wipeTransition.Metadata()
	if metadata != nil {
		if xges, ok := metadata["xges"].(map[string]interface{}); ok {
			if childProps, ok := xges["children-properties"].(string); ok {
				if !strings.Contains(childProps, "GESVideoTransition::border=(uint)0") {
					t.Errorf("Expected children-properties to contain border param, got '%s'", childProps)
				}
			}
		}
	}
}

func TestEncoder_GeneratorReference(t *testing.T) {
	timeline := opentimelineio.NewTimeline("Test", nil, nil)
	videoTrack := opentimelineio.NewTrack("video", nil, opentimelineio.TrackKindVideo, nil, nil)

	// Test pattern clip
	rate := 25.0
	genRef := opentimelineio.NewGeneratorReference("", "bars", nil, nil, nil)
	sourceRange := opentime.NewTimeRange(
		opentime.NewRationalTime(0, rate),
		opentime.NewRationalTime(25, rate),
	)
	testClip := opentimelineio.NewClip("test-pattern", genRef, &sourceRange, nil, nil, nil, "", nil)
	videoTrack.AppendChild(testClip)

	// Title clip
	titleRef := opentimelineio.NewGeneratorReference("", "title", nil, nil, nil)
	titleRange := opentime.NewTimeRange(
		opentime.NewRationalTime(0, rate),
		opentime.NewRationalTime(50, rate),
	)
	titleClip := opentimelineio.NewClip("title", titleRef, &titleRange, nil, nil, nil, "", nil)

	// Add text metadata
	titleMetadata := map[string]interface{}{
		"xges": map[string]interface{}{
			"text": "Test Title",
		},
	}
	titleClip.SetMetadata(titleMetadata)
	videoTrack.AppendChild(titleClip)

	timeline.Tracks().AppendChild(videoTrack)

	// Encode
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err := encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	output := buf.String()

	// Verify test clip
	if !strings.Contains(output, `type-name="GESTestClip"`) {
		t.Error("Output missing GESTestClip type")
	}
	if !strings.Contains(output, `asset-id="bars"`) {
		t.Error("Output missing bars asset-id")
	}

	// Verify title clip
	if !strings.Contains(output, `type-name="GESTitleClip"`) {
		t.Error("Output missing GESTitleClip type")
	}
	if !strings.Contains(output, `Test\ Title`) {
		t.Error("Output missing title text")
	}
}

func TestRoundTrip_NewFeatures(t *testing.T) {
	// Decode with new clip types
	decoder := NewDecoder(strings.NewReader(testClipsXGES))
	timeline, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Encode it back
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err = encoder.Encode(timeline)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode again
	decoder2 := NewDecoder(&buf)
	timeline2, err := decoder2.Decode()
	if err != nil {
		t.Fatalf("Second decode failed: %v", err)
	}

	// Verify both timelines have the same structure
	videoTracks1 := timeline.VideoTracks()
	videoTracks2 := timeline2.VideoTracks()

	if len(videoTracks1) != len(videoTracks2) {
		t.Errorf("Video track counts differ: %d vs %d", len(videoTracks1), len(videoTracks2))
	}

	if len(videoTracks1) > 0 {
		children1 := videoTracks1[0].Children()
		children2 := videoTracks2[0].Children()

		clipCount1 := 0
		clipCount2 := 0
		for _, c := range children1 {
			if _, ok := c.(*opentimelineio.Clip); ok {
				clipCount1++
			}
		}
		for _, c := range children2 {
			if _, ok := c.(*opentimelineio.Clip); ok {
				clipCount2++
			}
		}

		if clipCount1 != clipCount2 {
			t.Errorf("Clip counts differ: %d vs %d", clipCount1, clipCount2)
		}
	}
}

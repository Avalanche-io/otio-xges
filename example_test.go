// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package xges

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Avalanche-io/gotio/opentime"
	"github.com/Avalanche-io/gotio"
)

// Example of decoding an XGES file
func ExampleDecoder_Decode() {
	xgesData := `<?xml version="1.0" ?>
<ges version='0.3'>
  <project properties='properties;' metadatas='metadatas, name=(string)"Example\ Project";'>
    <timeline properties='properties;' metadatas='metadatas, framerate=(fraction)24/1;'>
      <track caps='video/x-raw(ANY)' track-type='4' track-id='0' properties='properties, restriction-caps=(string)"video/x-raw\,\ framerate\=\(fraction\)24/1";'/>
      <layer priority='0'>
        <clip id='0' asset-id='file:///example.mp4' type-name='GESUriClip' layer-priority='0' track-types='4' start='0' duration='2000000000' inpoint='0' rate='0' properties='properties, name=(string)"MyClip";' />
      </layer>
    </timeline>
  </project>
</ges>`

	decoder := NewDecoder(strings.NewReader(xgesData))
	timeline, err := decoder.Decode()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Timeline: %s\n", timeline.Name())
	fmt.Printf("Video tracks: %d\n", len(timeline.VideoTracks()))

	// Output:
	// Timeline: Example Project
	// Video tracks: 1
}

// Example of encoding a timeline to XGES
func ExampleEncoder_Encode() {
	// Create a timeline
	timeline := gotio.NewTimeline("My Edit", nil, nil)

	// Create a video track
	videoTrack := gotio.NewTrack("V1", nil, gotio.TrackKindVideo, nil, nil)

	// Add a clip
	rate := 24.0
	mediaRef := gotio.NewExternalReference("", "file:///media/clip001.mov", nil, nil)
	sourceRange := opentime.NewTimeRange(
		opentime.NewRationalTime(0, rate),
		opentime.NewRationalTime(120, rate), // 5 seconds at 24fps
	)
	clip := gotio.NewClip("shot_010", mediaRef, &sourceRange, nil, nil, nil, "", nil)

	videoTrack.AppendChild(clip)
	timeline.Tracks().AppendChild(videoTrack)

	// Encode to XGES
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	err := encoder.Encode(timeline)
	if err != nil {
		panic(err)
	}

	// Check output contains expected elements
	output := buf.String()
	fmt.Printf("Contains <ges>: %v\n", strings.Contains(output, "<ges"))
	fmt.Printf("Contains track: %v\n", strings.Contains(output, "track-type=\"4\""))
	fmt.Printf("Contains clip: %v\n", strings.Contains(output, "file:///media/clip001.mov"))

	// Output:
	// Contains <ges>: true
	// Contains track: true
	// Contains clip: true
}

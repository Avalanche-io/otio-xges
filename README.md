# XGES Adapter for Go OpenTimelineIO

A Go implementation of the XGES (GStreamer Editing Services) adapter for OpenTimelineIO.

## Overview

This package provides encoding and decoding support for XGES XML files, enabling conversion between OpenTimelineIO timelines and GStreamer Editing Services projects.

## Features

- **Decoder**: Parse XGES XML files into OTIO Timeline objects
- **Encoder**: Write OTIO Timeline objects as XGES XML
- Support for video and audio tracks
- Clip, gap, and transition handling
- Frame rate detection and conversion
- GStreamer structure string escaping/unescaping
- Timeline metadata preservation

## Installation

```bash
go get github.com/mrjoshuak/gotio/otio-xges
```

## Usage

### Decoding XGES to OTIO

```go
package main

import (
    "os"
    "github.com/mrjoshuak/gotio/otio-xges"
)

func main() {
    // Open XGES file
    f, err := os.Open("timeline.xges")
    if err != nil {
        panic(err)
    }
    defer f.Close()

    // Create decoder and decode
    decoder := xges.NewDecoder(f)
    timeline, err := decoder.Decode()
    if err != nil {
        panic(err)
    }

    // Use the timeline
    println("Timeline:", timeline.Name())
    println("Video tracks:", len(timeline.VideoTracks()))
    println("Audio tracks:", len(timeline.AudioTracks()))
}
```

### Encoding OTIO to XGES

```go
package main

import (
    "os"
    "github.com/mrjoshuak/gotio/opentimelineio"
    "github.com/mrjoshuak/gotio/otio-xges"
)

func main() {
    // Create or load a timeline
    timeline := opentimelineio.NewTimeline("My Timeline", nil, nil)

    // ... add tracks and clips to timeline ...

    // Open output file
    f, err := os.Create("output.xges")
    if err != nil {
        panic(err)
    }
    defer f.Close()

    // Create encoder and encode
    encoder := xges.NewEncoder(f)
    err = encoder.Encode(timeline)
    if err != nil {
        panic(err)
    }
}
```

## XGES Format

The XGES format is an XML-based representation of GStreamer Editing Services timelines:

- **Root element**: `<ges>` with version attribute
- **Project**: Contains timeline and resources
- **Timeline**: Contains tracks and layers
- **Tracks**: Video (track-type=4) and Audio (track-type=2)
- **Layers**: Contain clips arranged sequentially
- **Clips**: URI clips, transition clips, etc.
- **Time units**: Nanoseconds (1 second = 1,000,000,000 nanoseconds)

## Supported Elements

### Supported
- GESUriClip → OTIO Clip with ExternalReference
- GESTransitionClip → OTIO Transition
- Video and Audio tracks
- Frame rate detection and conversion
- Timeline and clip metadata
- Gaps (implicit in XGES, explicit in OTIO)

### Not Yet Supported
- GESTestClip (generator clips)
- GESTitleClip (title clips)
- GESOverlayClip (overlay clips)
- Effect bindings and property animations
- Nested timelines/sub-projects
- Asset metadata and stream info
- Groups

## Testing

Run the test suite:

```bash
go test -v
```

## API

### Decoder

```go
type Decoder struct { ... }

func NewDecoder(r io.Reader) *Decoder
func (d *Decoder) Decode() (*opentimelineio.Timeline, error)
```

### Encoder

```go
type Encoder struct { ... }

func NewEncoder(w io.Writer) *Encoder
func (e *Encoder) Encode(t *opentimelineio.Timeline) error
```

## License

Apache 2.0 - See LICENSE file for details

## Contributing

Contributions welcome! This adapter is part of the Go OpenTimelineIO project.

## Reference

Based on the Python XGES adapter: https://github.com/OpenTimelineIO/otio-xges-adapter

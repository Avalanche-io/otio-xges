// SPDX-License-Identifier: Apache-2.0
// Copyright Contributors to the OpenTimelineIO project

package xges

import "encoding/xml"

// GES represents the root element of an XGES file
type GES struct {
	XMLName xml.Name `xml:"ges"`
	Version string   `xml:"version,attr"`
	Project Project  `xml:"project"`
}

// Project represents the project element
type Project struct {
	Properties string   `xml:"properties,attr,omitempty"`
	Metadatas  string   `xml:"metadatas,attr,omitempty"`
	Timeline   Timeline `xml:"timeline"`
}

// Timeline represents the timeline element
type Timeline struct {
	Properties string  `xml:"properties,attr,omitempty"`
	Metadatas  string  `xml:"metadatas,attr,omitempty"`
	Tracks     []Track `xml:"track"`
	Layers     []Layer `xml:"layer"`
}

// Track represents a track element (video/audio)
type Track struct {
	Caps       string `xml:"caps,attr"`
	TrackType  int    `xml:"track-type,attr"`
	TrackID    int    `xml:"track-id,attr"`
	Properties string `xml:"properties,attr,omitempty"`
	Metadatas  string `xml:"metadatas,attr,omitempty"`
}

// Layer represents a layer element
type Layer struct {
	Priority   int    `xml:"priority,attr"`
	Properties string `xml:"properties,attr,omitempty"`
	Metadatas  string `xml:"metadatas,attr,omitempty"`
	Clips      []Clip `xml:"clip"`
}

// Clip represents a clip element
type Clip struct {
	ID                 int    `xml:"id,attr"`
	AssetID            string `xml:"asset-id,attr"`
	TypeName           string `xml:"type-name,attr"`
	LayerPriority      int    `xml:"layer-priority,attr"`
	TrackTypes         int    `xml:"track-types,attr"`
	Start              uint64 `xml:"start,attr"`
	Duration           uint64 `xml:"duration,attr"`
	Inpoint            uint64 `xml:"inpoint,attr"`
	Rate               int    `xml:"rate,attr"`
	Properties         string `xml:"properties,attr,omitempty"`
	Metadatas          string `xml:"metadatas,attr,omitempty"`
	ChildrenProperties string `xml:"children-properties,attr,omitempty"`
}

// Clip type names
const (
	ClipTypeURI        = "GESUriClip"
	ClipTypeTransition = "GESTransitionClip"
	ClipTypeTest       = "GESTestClip"
	ClipTypeTitle      = "GESTitleClip"
)

// Track types (as bitmask)
const (
	TrackTypeUnknown = 1 << 0
	TrackTypeAudio   = 1 << 1
	TrackTypeVideo   = 1 << 2
	TrackTypeText    = 1 << 3
	TrackTypeCustom  = 1 << 4
)

// GStreamer time is in nanoseconds
const GSTSecond = 1000000000

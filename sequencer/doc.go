// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// Package sequencer provides a tick-based pattern playback engine for
// scheduling musical events over time. Patterns are built using a fluent
// builder API and assigned to channels. The sequencer advances in sample-
// accurate increments, triggering NoteOn and NoteOff events through the
// voice manager at the correct musical positions.
package sequencer

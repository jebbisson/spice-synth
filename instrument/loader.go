// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrument

import (
	"fmt"
	"strings"

	"github.com/jebbisson/spice-synth/voice"
)

// Resolve returns the first voice.Instrument variant for a named instrument.
func (f *File) Resolve(name string) (*voice.Instrument, error) {
	variants, err := f.ResolveAll(name)
	if err != nil {
		return nil, err
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("instrument: no variants found for %q", name)
	}
	return variants[0], nil
}

// ResolveAll returns all voice.Instrument variants for a named instrument.
func (f *File) ResolveAll(name string) ([]*voice.Instrument, error) {
	var result []*voice.Instrument
	for _, fi := range f.Instruments {
		if strings.EqualFold(fi.Name, name) {
			for _, v := range fi.Variants {
				result = append(result, v.ToInstrumentWithParent(fi.Name))
			}
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("instrument: instrument %q not found", name)
	}
	return result, nil
}

// VariantKey returns "instrument_name.variant_name" for lookup.
func (f *File) VariantKey(instName, variantName string) string {
	return fmt.Sprintf("%s.%s", instName, variantName)
}

// FindByVariantKey finds an instrument by its full variant key.
// The key format is "instrument_name.variant_name".
func (f *File) FindByVariantKey(key string) (*voice.Instrument, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("instrument: invalid variant key %q (expected name.variant)", key)
	}
	instName, variantName := parts[0], parts[1]

	for _, fi := range f.Instruments {
		if strings.EqualFold(fi.Name, instName) {
			for _, v := range fi.Variants {
				if strings.EqualFold(v.Name, variantName) {
					return v.ToInstrumentWithParent(fi.Name), nil
				}
			}
		}
	}
	return nil, fmt.Errorf("instrument: variant %q not found", key)
}

// Groups returns the set of group names defined in the file.
func (f *File) Groups() []string {
	groupSet := make(map[string]bool)
	for _, fi := range f.Instruments {
		if fi.Group != "" {
			groupSet[fi.Group] = true
		}
	}
	groups := make([]string, 0)
	for g := range groupSet {
		groups = append(groups, g)
	}
	return groups
}

// LoadGroup loads only instruments in a specific group into a new File.
func (f *File) LoadGroup(group string) *File {
	result := &File{}
	for _, fi := range f.Instruments {
		if fi.Group == group {
			result.Instruments = append(result.Instruments, fi)
		}
	}
	return result
}

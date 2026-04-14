## ADL Buffered OPL Writes

### Problem

`examples/adl_player` exposed a playback issue in `DUNE1.ADL` subsong `6` where a fast repeated percussive/melodic part on channel `0` and instrument `143` became much quieter over time than expected.

The visible symptom was:

- the part started audible
- repeated fast beats lost impact over time
- soloing the channel made it clear the part was not pulsing as strongly as expected

### Investigation Summary

During diagnosis, the following were checked:

- ADL channel state such as instrument id, note, duration, and volume modifiers
- repeated-note retrigger behavior in the ADL driver
- soloed channel output compared to the mixed output
- per-channel visualization and metering in the debug player

The most important conclusion was that the remaining mismatch was in the actual playback path, not just the visualization.

### Root Cause

The ADL driver was writing OPL registers with immediate writes:

- `OPL3_WriteReg`

For dense, rapid ADL note traffic, this did not match the hardware-like register timing that Nuked-OPL3 expects for correct behavior.

Nuked-OPL3 provides a buffered write path:

- `OPL3_WriteRegBuffered`

Using the immediate path in the ADL driver caused fast repeated notes to lose the intended attack/behavior over time.

### Actual Fix

The ADL driver's register write helper was changed to use buffered OPL writes instead of immediate writes.

Changed location:

- `adl/driver.go`
- `func (d *Driver) writeOPL(reg, val uint8)`

Supporting wrapper added:

- `chip/chip.go`
- `func (o *OPL3) WriteRegisterBuffered(...)`

Follow-up consistency fix:

- `voice/voice.go`
- DSL/stream playback now uses the same buffered write path for note, instrument,
  and modulation register updates so freshly extracted ADL-to-DSL songs do not
  diverge from the corrected live ADL playback timing.

### Why This Fix Is Correct

Buffered writes more closely match the timing behavior that the emulated chip expects.

After this change:

- the problematic DUNE1 subsong behaved correctly in the debug player
- the identified channel stayed strong over time
- the repeated beats regained the expected pulse/impact

### Notes About Other Changes During Diagnosis

Several debugging-oriented changes were made while narrowing down the issue, including:

- extra channel snapshot/debug data
- temporary diagnostic tests for DUNE1 subsong 6
- `adl_player` visualization and solo controls

The actual playback fix was the buffered-write change described above.

### Verification

Validated with:

- focused DUNE1 subsong 6 regression coverage in `adl/adl_test.go`
- `go test ./...`
- manual run of `examples/adl_player` on `DUNE1.ADL` subsong `6`

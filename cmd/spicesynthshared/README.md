# spicesynthshared

`spicesynthshared` builds SpiceSynth itself as a C-compatible shared library or
linkable archive.

## Build as Shared Library

```bash
go build -buildmode=c-shared -o libspicesynth.so ./cmd/spicesynthshared
```

Common outputs by platform:

- Linux: `libspicesynth.so`
- macOS: `libspicesynth.dylib`
- Windows: `spicesynth.dll`

This build also emits a generated C header next to the library artifact.

## Build as Linkable Archive

```bash
go build -buildmode=c-archive -o libspicesynth.a ./cmd/spicesynthshared
```

This build emits:

- `libspicesynth.a`
- a generated C header

Use this mode when another product wants to link SpiceSynth into its own binary
while still keeping a relinkable deliverable.

## Exported API

Streams:

- `SpiceSynth_Stream_Create`
- `SpiceSynth_Stream_Destroy`
- `SpiceSynth_Stream_Read`

Players:

- `SpiceSynth_Player_CreateMIDI`
- `SpiceSynth_Player_CreateADL`
- `SpiceSynth_Player_Destroy`
- `SpiceSynth_Player_Play`
- `SpiceSynth_Player_Pause`
- `SpiceSynth_Player_Stop`
- `SpiceSynth_Player_GetState`
- `SpiceSynth_Player_SetSubsong`
- `SpiceSynth_Player_NumSubsongs`
- `SpiceSynth_Player_Read`

## Packaging Another Project

### Shared library mode

1. Build `spicesynthshared` with `-buildmode=c-shared`
2. Bundle the generated library with your application
3. Bundle the generated header if your build system compiles against the C ABI
4. Include `LICENSE` and `THIRD_PARTY_LICENSES`

### Linkable archive mode

1. Build `spicesynthshared` with `-buildmode=c-archive`
2. Link the produced archive into your product build
3. Keep the generated header with your build inputs
4. Preserve a relinkable path for the archive deliverable in your packaging process

## Notes

- The repository does not commit prebuilt binaries
- You should build platform-specific artifacts in your product pipeline
- The shared library / archive is the recommended distribution mode for products
  that want a straightforward LGPL packaging story
- ADL song conversion and ADL instrument YAML extraction are provided by the
  separate `cmd/adl2dsl` CLI, not by the shared library ABI

// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package adl

// opcodeParamCount maps ADL opcode indexes to parameter counts. This remains
// local because the parser-side subsong classification logic uses it directly.
var opcodeParamCount = [75]int{
	1, 2, 1, 1, 2, 2, 0, 1, 0, 1,
	2, 2, 1, 5, 1, 1, 1, 3, 0, 1,
	0, 4, 0, 0, 0, 0, 1, 0, 1, 1,
	1, 0, 1, 1, 0, 0, 1, 0, 1, 0,
	0, 1, 0, 1, 2, 2, 1, 1, 1, 0,
	0, 1, 0, 2, 0, 0, 0, 1, 0, 0,
	1, 1, 0, 2, 0, 9, 1, 0, 2, 2,
	2, 1, 0, 0, 0,
}

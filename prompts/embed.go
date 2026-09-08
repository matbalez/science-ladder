package prompts

import _ "embed"

// Scout is a versioned portable prompt, not an instruction to publish automatically.
//
//go:embed challenge-scout-v1.4.md
var Scout string

//go:embed challenge-scout-v1.3.md
var ScoutV13 string

//go:embed challenge-scout-v1.2.md
var ScoutV12 string

//go:embed challenge-scout-v1.md
var ScoutV11 string

//go:embed challenge-scout-v1.0.md
var ScoutV10 string

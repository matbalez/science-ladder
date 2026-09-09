package prompts

import _ "embed"

// Scout is a versioned portable prompt, not an instruction to publish automatically.
//
//go:embed challenge-scout-v1.9.md
var Scout string

//go:embed challenge-scout-v1.8.md
var ScoutV18 string

//go:embed challenge-scout-v1.7.md
var ScoutV17 string

//go:embed challenge-scout-v1.6.md
var ScoutV16 string

//go:embed challenge-scout-v1.5.md
var ScoutV15 string

//go:embed challenge-scout-v1.4.md
var ScoutV14 string

//go:embed challenge-scout-v1.3.md
var ScoutV13 string

//go:embed challenge-scout-v1.2.md
var ScoutV12 string

//go:embed challenge-scout-v1.md
var ScoutV11 string

//go:embed challenge-scout-v1.0.md
var ScoutV10 string

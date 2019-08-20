package version

const Logo = `

 ___      ___ ______    ___    __ ______   ___   ____
|   \    /  _]      |  /  _]  /  ]      | /   \ |    \
|    \  /  [_|      | /  [_  /  /|      ||     ||  D  )
|  D  ||    _]_|  |_||    _]/  / |_|  |_||  O  ||    /
|     ||   [_  |  |  |   [_/   \_  |  |  |     ||    \
|     ||     | |  |  |     \     | |  |  |     ||  .  \
|_____||_____| |__|  |_____|\____| |__|   \___/ |__|\_|
`

const Mark = `+----------------------+------------------------------------------+`

// These variables are populated via the Go linker.
var (
	UTCBuildTime  = "unknown"
	ClientVersion = "unknown"
	GoVersion     = "unknown"
	GitBranch     = "unknown"
	GitTag        = "unknown"
	GitHash       = "unknown"
)

module c64dpainter

go 1.22

require (
	github.com/ebitengine/purego v0.10.0
	github.com/jezek/xgb v1.3.1
)

replace github.com/ebitengine/purego => ./third_party/purego

replace github.com/jezek/xgb => ./third_party/xgb

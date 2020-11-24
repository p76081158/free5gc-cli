package ngapType

import "free5gc-cli/lib/aper"

// Need to import "free5gc-cli/lib/aper" if it uses "aper"

const (
	DelayCriticalPresentDelayCritical    aper.Enumerated = 0
	DelayCriticalPresentNonDelayCritical aper.Enumerated = 1
)

type DelayCritical struct {
	Value aper.Enumerated `aper:"valueExt,valueLB:0,valueUB:1"`
}
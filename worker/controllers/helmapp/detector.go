package helmapp

type Detector struct {
	status *Status
}

func NewDetector(status *Status) *Detector {
	return &Detector{
		status: status,
	}
}

func (d *Detector) Detect() error {
	if d.status.isDetected() {
		return nil
	}

	// add repo

	return nil
}

package vmrestore

// ClaimDeliveryCapture shares the existing durable admission with power, disk,
// restore and recycle operations. Ownership never expires into permission.
func (s *Store) ClaimDeliveryCapture(scope Scope, captureID string) error {
	if !idPattern.MatchString(captureID) {
		return ErrCorrupt
	}
	return s.deliveryCaptureMutation(scope, captureID, false)
}

// ReleaseDeliveryCapture is called only after the export and its readers are gone.
func (s *Store) ReleaseDeliveryCapture(scope Scope, captureID string) error {
	if !idPattern.MatchString(captureID) {
		return ErrCorrupt
	}
	return s.deliveryCaptureMutation(scope, captureID, true)
}

func (s *Store) deliveryCaptureMutation(scope Scope, captureID string, release bool) error {
	current, err := s.Read(scope, "")
	if err != nil {
		return err
	}
	if release && current.State.Admission != nil && (current.State.Admission.Token != captureID || current.State.Admission.Action != "delivery-capture") {
		return ErrConflict
	}
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	_, err = s.MutateTx(tx, scope, Expectation{AdmissionOwner: captureID, ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(now Current) (Mutation, error) {
		admission := now.State.Admission
		if release {
			if admission != nil {
				if admission.Action != "delivery-capture" || admission.Token != captureID {
					return Mutation{}, ErrConflict
				}
				if err := now.State.FinishMutation(captureID, true); err != nil {
					return Mutation{}, err
				}
			}
		} else if admission == nil {
			if err := now.State.AdmitMutation(captureID, "delivery-capture"); err != nil {
				return Mutation{}, err
			}
		} else if admission.Token != captureID || admission.Action != "delivery-capture" || now.State.Fence != nil {
			return Mutation{}, ErrBusy
		}
		return Mutation{State: now.State}, nil
	})
	if err != nil {
		return err
	}
	return tx.Commit().Error
}

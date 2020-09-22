package conversion

import (
	"fmt"

	"github.com/goodrain/rainbond/db"
)

// startupSequenceDetector can detect whether the service require a startup sequence,
// and set dependent services for it.
type startupSequenceDetector struct {
	serviceID string

	dbmanager db.Manager
}

func newStartupSequenceDetector(serviceID string, dbmanager db.Manager) *startupSequenceDetector {
	return &startupSequenceDetector{
		serviceID: serviceID,
		dbmanager: dbmanager,
	}
}

func (s *startupSequenceDetector) dependServices() (string, int, error) {
	depServiceIDs, err := s.directlyDepServices(s.serviceID)
	if err != nil {
		return "", 0, err
	}

	var serviceIDs []string
	for _, depServiceID := range depServiceIDs {
		ok, err := s.dependOnEachOther(depServiceID)
		if err != nil {
			return "", 0, err
		}
		// Ignore interdependent components
		if ok {
			continue
		}
		serviceIDs = append(serviceIDs, depServiceID)
	}

	// build result: alias1:id2,alias2:id2,alias2:id2
	serivces, err := s.dbmanager.TenantServiceDao().GetServiceAliasByIDs(serviceIDs)
	if err != nil {
		return "", 0, err
	}
	var res string
	for _, svc := range serivces {
		if res != "" {
			res += ","
		}
		res += fmt.Sprintf("%s:%s", svc.ServiceAlias, svc.ServiceID)
	}

	return res, len(serviceIDs), nil
}

// directlyDepServices get directly dependent services
func (s *startupSequenceDetector) directlyDepServices(serviceID string) ([]string, error) {
	relations, err := s.dbmanager.TenantServiceRelationDao().GetTenantServiceRelations(serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service relations: %v", err)
	}

	var res []string
	for _, r := range relations {
		res = append(res, r.DependServiceID)
	}
	return res, nil
}

// dependOnEachOther recursively checks whether the dependent components depend on each other.
// s.serviceID: original serviceID.
// serviceID: serviceID of the current layer, child of s.serviceID.
func (s *startupSequenceDetector) dependOnEachOther(serviceID string) (bool, error) {
	// serviceIDs that serviceID depends on.
	depServiceIDs, err := s.directlyDepServices(serviceID)
	if err != nil {
		return false, err
	}

	for _, depServiceID := range depServiceIDs {
		// Formed a ring, indicating mutual dependence
		if s.serviceID == depServiceID {
			return true, nil
		}
		// go further
		ok, err := s.dependOnEachOther(depServiceID)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

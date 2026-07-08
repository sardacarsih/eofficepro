package handler

var positionTypesByUnitLevel = map[string]map[string]bool{
	"office": {
		"president_director": true,
		"vp_director":        true,
		"assistant":          true,
		"staff":              true,
	},
	"directorate": {
		"director":  true,
		"secretary": true,
		"auditor":   true,
		"assistant": true,
		"staff":     true,
	},
	"biro": {
		"gm":        true,
		"secretary": true,
		"assistant": true,
		"staff":     true,
	},
	"department": {
		"dept_head":     true,
		"sub_dept_head": true,
		"staff":         true,
	},
	"division": {
		"division_head": true,
		"staff":         true,
	},
}

func validPositionTypeForUnitLevel(unitLevel, positionType string) bool {
	allowed, ok := positionTypesByUnitLevel[unitLevel]
	if !ok {
		return false
	}
	return allowed[positionType]
}

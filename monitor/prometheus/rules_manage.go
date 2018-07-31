package prometheus


type AlertingRulesConfig struct {
	Groups []*AlertingNameConfig `yaml:"groups"`
}

type AlertingNameConfig struct {
	Name  string         `yaml:"name"`
	Rules []*RulesConfig `yaml:"rules"`
}

type RulesConfig struct {
	Alert  string            `yaml:"alert"`
	Expr   string            `yaml:"expr"`
	For    string            `yaml:"for"`
	Labels map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func NewR()  {
	
}
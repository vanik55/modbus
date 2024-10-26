package modbus

import "strconv"

type Value struct {
	ValueCode  string
	Parameters map[string]interface{}
}

// AddParametr adds a parameter to the value
func (v *Value) AddParameter(key string, value interface{}) {
	v.Parameters[key] = value
}

func (v *Value) GetStringParameter(key string) string {
	value, ok := v.Parameters[key]
	if ok {
		return value.(string)
	}
	return ""
}

func (v *Value) GetStringParameterDefault(key, defaultValue string) string {
	value, ok := v.Parameters[key]
	if ok {
		return value.(string)
	} else {
		return defaultValue
	}
}

func (v *Value) GetIntParameter(key string) int {
	return v.GetIntParameterDefault(key, 0)
}

func (v *Value) GetIntParameterDefault(key string, defaultValue int) int {
	value, ok := v.Parameters[key]
	if ok {
		intVal, ok := value.(int)
		if ok {
			return intVal
		} else {
			intVal, _ := strconv.Atoi(value.(string))
			return intVal
		}
	}
	return defaultValue
}

func (v *Value) GetFloatParameter(key string) float64 {
	return v.GetFloatParameterDefault(key, 0.0)
}

func (v *Value) GetFloatParameterDefault(key string, defaultValue float64) float64 {
	value, ok := v.Parameters[key]
	if ok {
		floatVal, ok := value.(float64)
		if ok {
			return floatVal
		} else {
			floatVal, _ := strconv.ParseFloat(value.(string), 64)
			return floatVal
		}
	}
	return defaultValue
}

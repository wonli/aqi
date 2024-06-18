package aqi

type Aqi struct {
	AppConfig *AppConfig
}

func Use() *Aqi {
	return &Aqi{
		AppConfig: acf,
	}
}

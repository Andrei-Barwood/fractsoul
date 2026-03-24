package efficiency

import "testing"

func TestComputeJTH(t *testing.T) {
	value := ComputeJTH(3500, 200)
	if value != 17.5 {
		t.Fatalf("expected 17.5, got %v", value)
	}
}

func TestCompensateJTH(t *testing.T) {
	baseline := BaselineForModel("S21")
	compensated := CompensateJTH(18, 35, baseline)
	if compensated >= 18 {
		t.Fatalf("expected compensated jth lower than raw when ambient is high, got %v", compensated)
	}
}

func TestClassifyThermalBand(t *testing.T) {
	baseline := BaselineForModel("S21")
	if band := ClassifyThermalBand(72, baseline); band != "optimal" {
		t.Fatalf("expected optimal, got %s", band)
	}
	if band := ClassifyThermalBand(98, baseline); band != "hotspot" {
		t.Fatalf("expected hotspot, got %s", band)
	}
}

func TestParseAmbient(t *testing.T) {
	value := ParseAmbient(map[string]string{"ambient_temp_c": "31.2"}, 25)
	if value != 31.2 {
		t.Fatalf("expected 31.2 got %v", value)
	}
}

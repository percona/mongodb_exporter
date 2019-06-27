package mongod

import (
	"io/ioutil"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func Test_isPow2(t *testing.T) {
	//edge case
	if isPow2(0) != false {
		t.Error("isPow2() failed for input 0")
	}

	//power of 2 cases
	for i := uint64(1); i < 1024; i = i * 2 {
		if isPow2(i) != true {
			t.Errorf("isPow2() failed for input %v", i)
		}
	}

	//non power of 2 cases
	for i := uint64(3); i < 300; i = i * 3 {
		if isPow2(i) != false {
			t.Errorf("isPow2() failed for input %v", i)
		}
	}
}

func Test_histMicrosEdgeToMidpoint(t *testing.T) {
	//edge cases
	if histMicrosEdgeToMidpoint(0) != 0.5 {
		t.Error("Wrong histogram midpoint")
	}
	if histMicrosEdgeToMidpoint(-1) != 0.5 {
		t.Error("Wrong histogram midpoint")
	}
	if histMicrosEdgeToMidpoint(1) != 0.5 {
		t.Error("Wrong histogram midpoint")
	}

	//cases between 1 and 2048
	for startEdge, endEdge := int64(1), int64(2); endEdge <= 2048; startEdge, endEdge = endEdge, endEdge*2 {
		midpoint := histMicrosEdgeToMidpoint(endEdge)
		expectedMidpoint := float64(endEdge-startEdge)/2.0 + float64(startEdge)
		if midpoint != float64(expectedMidpoint) {
			t.Errorf("Wrong histogram midpoint.  Expected: %v, Found: %v", expectedMidpoint, midpoint)
		}
	}

	//a few cases after 2048
	for startEdge, endEdge := int64(2048), int64(4096); endEdge <= 32768; startEdge, endEdge = endEdge, endEdge*2 {
		halfwayEdge := (endEdge-startEdge)/2 + startEdge

		midpoint1 := histMicrosEdgeToMidpoint(halfwayEdge)
		expectedMidpoint1 := float64(halfwayEdge-startEdge)/2.0 + float64(startEdge)
		midpoint2 := histMicrosEdgeToMidpoint(endEdge)
		expectedMidpoint2 := float64(endEdge-halfwayEdge)/2.0 + float64(halfwayEdge)

		if midpoint1 != float64(expectedMidpoint1) {
			t.Errorf("Wrong histogram midpoint.  Expected: %v, Found: %v", expectedMidpoint1, midpoint1)
		}
		if midpoint2 != float64(expectedMidpoint2) {
			t.Errorf("Wrong histogram midpoint.  Expected: %v, Found: %v", expectedMidpoint2, midpoint2)
		}
	}
}

func Test_clipObservationCount(t *testing.T) {
	data := loadFixture("op_latencies.bson")

	var opLatencies OpLatenciesStat
	loadOpLatenciesFromBson(data, &opLatencies)

	// no previous histogram should pass through all input
	if clipObservationCount(nil, 0, 100) != int64(100) {
		t.Error("clipObservationCount failed")
	}

	// Data contains 4 counts, expected result is 0
	if clipObservationCount(opLatencies.Reads, 128, 4) != int64(0) {
		t.Error("clipObservationCount failed")
	}

	// Data contains 9427 counts
	if clipObservationCount(opLatencies.Reads, 262144, 10000) != int64(10000-9427) {
		t.Error("clipObservationCount failed")
	}

}

func loadOpLatenciesFromBson(data []byte, stat *OpLatenciesStat) {
	err := bson.Unmarshal(data, stat)
	if err != nil {
		panic(err)
	}
}

func loadFixture(name string) []byte {
	data, err := ioutil.ReadFile("../fixtures/" + name)
	if err != nil {
		panic(err)
	}

	return data
}

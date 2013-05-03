package CloudForest

import (
	"encoding/csv"
	"io"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
)

//FeatureMatrix contains a slice of Features and a Map to look of the index of a feature
//by its string id.
type FeatureMatrix struct {
	Data       []Feature
	Map        map[string]int
	CaseLabels []string
}

/*
BestSplitter finds the best splitter from a number of canidate features to slit on by looping over
all features and calling BestSplit.

Pointers to slices for l and r are used to reduce realocations during repeated calls
and will not contain meaningfull results.

l and r should have capacity >=  cap(cases) to avoid resizing.
*/
func (fm *FeatureMatrix) BestSplitter(target Target,
	cases []int,
	canidates []int,
	itter bool,
	splitmissing bool,
	l *[]int,
	r *[]int) (s *Splitter, impurityDecrease float64) {

	impurityDecrease = minImp

	var f, bestF *Feature
	var num, bestNum, inerImp float64
	var cat, bestCat int
	var bigCat, bestBigCat *big.Int

	sorter := new(SortableFeature)
	var counter []int

	ncats := target.NCats()

	if ncats > 0 {
		counter = make([]int, ncats, ncats)
	}

	parentImp := target.Impurity(&cases, &counter)

	left := *l
	right := *r

	for _, i := range canidates {
		left = left[:]
		right = right[:]
		f = &fm.Data[i]
		num, cat, bigCat, inerImp = f.BestSplit(target, &cases, parentImp, itter, splitmissing, &left, &right, &counter, sorter)
		//BUG more stringent cutoff in BestSplitter?
		if inerImp > minImp && inerImp > impurityDecrease {
			bestF = f
			impurityDecrease = inerImp
			bestNum = num
			bestCat = cat
			bestBigCat = bigCat
		}

	}
	if impurityDecrease > minImp {
		s = bestF.DecodeSplit(bestNum, bestCat, bestBigCat)
	}
	return
}

/*
AddContrasts appends n artificial contrast features to a feature matrix. These features
are generated by randomly selecting (with replacement) an existing feature and
creating a shuffled copy named featurename:SHUFFLED.

These features can be used as a contrast to evaluate the importance score's assigned to
actual features.
*/
func (fm *FeatureMatrix) AddContrasts(n int) {
	nrealfeatures := len(fm.Data)
	for i := 0; i < n; i++ {

		//generate a shuffled copy
		orig := fm.Data[rand.Intn(nrealfeatures)]
		fake := orig.ShuffledCopy()

		fm.Map[fake.Name] = len(fm.Data)

		fm.Data = append(fm.Data, *fake)

	}
}

/*
ContrastAll adds shuffled copies of every feature to the feature matrix. These features
are generated by randomly selecting (with replacement) an existing feature and
creating a shuffled copy named featurename:SHUFFLED.

These features can be used as a contrast to evaluate the importance score's assigned to
actual features. ContrastAll is particularly usefull vs AddContrast when one wishes to
identify [psuedo] unique identifiers that might lead to overfitting.
*/
func (fm *FeatureMatrix) ContrastAll() {
	nrealfeatures := len(fm.Data)
	for i := 0; i < nrealfeatures; i++ {

		fake := fm.Data[i].ShuffledCopy()

		fm.Map[fake.Name] = len(fm.Data)

		fm.Data = append(fm.Data, *fake)

	}
}

/*
ImputeMissing imputes missing values in all features to the mean or mode of the feature.
*/
func (fm *FeatureMatrix) ImputeMissing() {
	for _, f := range fm.Data {
		f.ImputeMissing()
	}
}

//Parse an AFM (anotated feature matrix) out of an io.Reader
//AFM format is a tsv with row and column headers where the row headers start with
//N: indicating numerical, C: indicating catagorical or B: indicating boolean
//For this parser features without N: are assumed to be catagorical
func ParseAFM(input io.Reader) *FeatureMatrix {
	data := make([]Feature, 0, 100)
	lookup := make(map[string]int, 0)
	tsv := csv.NewReader(input)
	tsv.Comma = '\t'
	headers, err := tsv.Read()
	if err == io.EOF {
		return &FeatureMatrix{data, lookup, headers[1:]}
	} else if err != nil {
		log.Print("Error:", err)
		return &FeatureMatrix{data, lookup, headers[1:]}
	}
	headers = headers[1:]

	count := 0
	for {
		record, err := tsv.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Print("Error:", err)
			break
		}
		data = append(data, ParseFeature(record))
		lookup[record[0]] = count
		count++
	}
	return &FeatureMatrix{data, lookup, headers}
}

//ParseFeature parses a Feature from an array of strings and a capacity
//capacity is the number of cases and will usually be len(record)-1 but
//but doesn't need to be calculated for every row of a large file.
//The type of the feature us infered from the start ofthe first (header) field
//in record:
//"N:"" indicating numerical, anything else (usually "C:" and "B:") for catagorical
func ParseFeature(record []string) Feature {
	capacity := len(record)
	f := Feature{
		&CatMap{make(map[string]int, 0),
			make([]string, 0, 0)},
		nil,
		nil,
		make([]bool, 0, capacity),
		false,
		record[0]}

	switch record[0][0:2] {
	case "N:":
		f.NumData = make([]float64, 0, capacity)
		//Numerical
		f.Numerical = true
		for i := 1; i < len(record); i++ {
			v, err := strconv.ParseFloat(record[i], 64)
			if err != nil {
				f.NumData = append(f.NumData, 0.0)
				f.Missing = append(f.Missing, true)
				continue
			}
			f.NumData = append(f.NumData, float64(v))
			f.Missing = append(f.Missing, false)

		}

	default:
		f.CatData = make([]int, 0, capacity)
		//Assume Catagorical
		f.Numerical = false
		for i := 1; i < len(record); i++ {
			v := record[i]
			norm := strings.ToLower(v)
			if norm == "?" || norm == "nan" || norm == "na" || norm == "null" {

				f.CatData = append(f.CatData, 0)
				f.Missing = append(f.Missing, true)
				continue
			}
			f.CatData = append(f.CatData, f.CatToNum(v))
			f.Missing = append(f.Missing, false)

		}

	}
	return f

}

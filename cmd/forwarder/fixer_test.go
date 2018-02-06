package main

import (
	"bytes"
	"testing"
)

type InputOutput struct {
	Input  []byte
	Output []byte
}

var (
	input = [][]byte{
		[]byte("64 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hi\n67 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hello\n"),
		[]byte("106 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n"),
		[]byte("65 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - hello\n"),
		[]byte("58 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - "),
		[]byte("97 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [60607e20-f12d-483e-aa89-ffaf954e7527]"),
	}
)

func TestFix(t *testing.T) {
	var output = [][]byte{
		[]byte("84 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"] hi\n87 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"] hello\n"),
		[]byte("127 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n"),
		[]byte("87 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"] hello\n"),
		[]byte("80 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"]"),
		[]byte("118 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][60607e20-f12d-483e-aa89-ffaf954e7527]"),
	}
	for x, in := range input {
		fixed, _ := fix(bytes.NewReader(in), "1.2.3.4", "", "")

		if !bytes.Equal(fixed, output[x]) {
			t.Errorf("input=%q\noutput=%q\ngot=%q\n", in, output[x], fixed)
		}
	}
}

func TestFixWithLogAuthUser(t *testing.T) {
	var output = [][]byte{
		[]byte("107 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"] hi\n110 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"] hello\n"),
		[]byte("150 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"][meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n"),
		[]byte("110 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"] hello\n"),
		[]byte("103 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"]"),
		[]byte("141 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [origin ip=\"1.2.3.4\"][log_iss user=\"ingest\"][60607e20-f12d-483e-aa89-ffaf954e7527]"),
	}
	for x, in := range input {
		fixed, _ := fix(bytes.NewReader(in), "1.2.3.4", "", "ingest")

		if !bytes.Equal(fixed, output[x]) {
			t.Errorf("input=%q\noutput=%q\ngot=%q\n", in, output[x], fixed)
		}
	}
}

func TestFixWithLogplexDrainToken(t *testing.T) {
	testToken := "d.34bc219c-983b-463e-a17d-3d34ee7db812"

	output := [][]byte{
		[]byte("118 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"] hi\n121 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"] hello\n"),
		[]byte("161 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"][meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n"),
		[]byte("121 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"] hello\n"),
		[]byte("114 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"]"),
		[]byte("152 <13>1 2013-06-07T13:17:49.468822+00:00 d.34bc219c-983b-463e-a17d-3d34ee7db812 heroku web.7 - [origin ip=\"1.2.3.4\"][60607e20-f12d-483e-aa89-ffaf954e7527]"),
	}

	for x, in := range input {
		fixed, _ := fix(bytes.NewReader(in), "1.2.3.4", testToken, "")

		if !bytes.Equal(fixed, output[x]) {
			t.Errorf("input=%q\noutput=%q\ngot=%q\n", in, output[x], fixed)
		}
	}
}

func BenchmarkFixNoSD(b *testing.B) {
	input := []byte("64 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hi\n67 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hello\n")
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fix(bytes.NewReader(input), "1.2.3.4", "", "")
	}
}

func BenchmarkFixSD(b *testing.B) {
	input := []byte("106 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n")
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fix(bytes.NewReader(input), "1.2.3.4", "", "")
	}
}

func BenchmarkLogAuthUserFixNoSD(b *testing.B) {
	input := []byte("64 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hi\n67 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - - hello\n")
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fix(bytes.NewReader(input), "1.2.3.4", "", "ingest")
	}
}

func BenchmarkLogAuthUserFixSD(b *testing.B) {
	input := []byte("106 <13>1 2013-06-07T13:17:49.468822+00:00 host heroku web.7 - [meta sequenceId=\"hello\"][foo bar=\"baz\"] hello\n")
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fix(bytes.NewReader(input), "1.2.3.4", "", "ingest")
	}
}

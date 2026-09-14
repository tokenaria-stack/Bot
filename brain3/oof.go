package brain3

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const (
	Metalabel1OOFDigest = "72bb757ec8802e18f67c4bfaa24b89344766de7e4f9f80f2cfd6084b9b918d59"
	Metalabel1OOFRows   = 4227
)

var Metalabel1FoldN = []int{1083, 1084, 1042, 1018}

type OOFFile struct {
	Format         string
	ClassSchema    []string
	Dataset        string
	Validation     string
	Split          string
	Spec           string
	Content        string
	Width          int
	HoldoutStartAt int64
	Rows           []OOFRow
	MaxAt          int64
}

func LoadOOF(path string) (OOFFile, error) {
	var z OOFFile
	f, err := os.Open(path)
	if err != nil {
		return z, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var sawFooter bool
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(line, &kind); err != nil {
			return z, err
		}
		switch kind.Kind {
		case "header":
			var h struct {
				Format       string   `json:"format"`
				ClassSchema  []string `json:"class_schema"`
				Dataset      string   `json:"dataset_digest"`
				Validation   string   `json:"validation_plan_digest"`
				Split        string   `json:"data_split_digest"`
				Spec         string   `json:"model_spec_digest"`
				Content      string   `json:"content_digest"`
				Width        int      `json:"width"`
				Rows         int      `json:"row_count"`
				HoldoutStart int64    `json:"holdout_start_at"`
			}
			if err := json.Unmarshal(line, &h); err != nil {
				return z, err
			}
			z.Format, z.ClassSchema = h.Format, append([]string(nil), h.ClassSchema...)
			z.Dataset, z.Validation, z.Split, z.Spec = h.Dataset, h.Validation, h.Split, h.Spec
			z.Content, z.Width, z.HoldoutStartAt = h.Content, h.Width, h.HoldoutStart
		case "row":
			var r struct {
				At     int64   `json:"at"`
				Fold   int     `json:"fold"`
				Y      int     `json:"y"`
				Logit0 float64 `json:"logit0"`
				Logit1 float64 `json:"logit1"`
				Logit2 float64 `json:"logit2"`
			}
			if err := json.Unmarshal(line, &r); err != nil {
				return z, err
			}
			z.Rows = append(z.Rows, OOFRow{At: r.At, Fold: r.Fold, Y: r.Y, Logit: [3]float64{r.Logit0, r.Logit1, r.Logit2}})
		case "footer":
			sawFooter = true
		default:
			return z, fmt.Errorf("brain3: unknown oof kind %q", kind.Kind)
		}
	}
	if err := sc.Err(); err != nil {
		return z, err
	}
	if !sawFooter {
		return z, fmt.Errorf("brain3: oof footer missing")
	}
	return z, z.validate(Metalabel1OOFDigest)
}

func (z *OOFFile) validate(expectDigest string) error {
	if z.Format != OOFFormatV1 {
		return fmt.Errorf("brain3: oof format %q", z.Format)
	}
	want := SetupClassSchema()
	if len(z.ClassSchema) != 3 || z.ClassSchema[0] != want[0] || z.ClassSchema[1] != want[1] || z.ClassSchema[2] != want[2] {
		return fmt.Errorf("brain3: class schema %v", z.ClassSchema)
	}
	if z.HoldoutStartAt <= 0 {
		return fmt.Errorf("brain3: holdout wall required")
	}
	if len(z.Rows) != Metalabel1OOFRows {
		return fmt.Errorf("brain3: oof rows %d != %d", len(z.Rows), Metalabel1OOFRows)
	}
	counts := map[int]int{}
	seen := map[int64]struct{}{}
	for i, r := range z.Rows {
		if r.Y < 0 || r.Y > 2 {
			return fmt.Errorf("brain3: y=%d", r.Y)
		}
		if r.At >= z.HoldoutStartAt {
			return fmt.Errorf("brain3: holdout At %d", r.At)
		}
		if _, ok := seen[r.At]; ok {
			return fmt.Errorf("brain3: duplicate OOF At %d", r.At)
		}
		seen[r.At] = struct{}{}
		if i > 0 && r.At <= z.Rows[i-1].At {
			return fmt.Errorf("brain3: OOF At not increasing")
		}
		if r.At > z.MaxAt {
			z.MaxAt = r.At
		}
		counts[r.Fold]++
	}
	if len(counts) != 4 {
		return fmt.Errorf("brain3: fold count %d", len(counts))
	}
	for i, n := range Metalabel1FoldN {
		if counts[i] != n {
			return fmt.Errorf("brain3: fold %d n=%d want %d", i, counts[i], n)
		}
	}
	hdr, err := json.Marshal(struct {
		Format, Spec, Dataset, Plan, Split string
		Schema                             []string
		Width                              int
	}{z.Format, z.Spec, z.Dataset, z.Validation, z.Split, z.ClassSchema, z.Width})
	if err != nil {
		return err
	}
	got := hashOOF(hdr, z.Rows)
	if got != z.Content {
		return fmt.Errorf("brain3: oof header digest %s != recomputed %s", z.Content, got)
	}
	if expectDigest != "" && got != expectDigest {
		return fmt.Errorf("brain3: oof digest %s != frozen %s", got, expectDigest)
	}
	return nil
}

func foldRows(rows []OOFRow) [][]OOFRow {
	mx := 0
	for _, r := range rows {
		if r.Fold > mx {
			mx = r.Fold
		}
	}
	out := make([][]OOFRow, mx+1)
	for _, r := range rows {
		out[r.Fold] = append(out[r.Fold], r)
	}
	for i := range out {
		sort.Slice(out[i], func(a, b int) bool { return out[i][a].At < out[i][b].At })
	}
	return out
}

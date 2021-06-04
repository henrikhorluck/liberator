package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	yaml2 "github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-tools/pkg/crd"
	crd_markers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// Generate documentation for NAIS CRD's

var defaultApplication interface{}

var exampleApplication = getExampleApp()

type Config struct {
	Directory       string
	BaseClass       string
	Group           string
	Kind            string
	ReferenceOutput string
	ExampleOutput   string
}

type Doc struct {
	// Which cluster(s) or environments the feature is available in
	Availability string `marker:"Availability,optional"`
	// Links to documentation or other information
	Link []string `marker:"Link,optional"`
}

type ExtDoc struct {
	Availability string
	Default      string
	Description  string
	Enum         []string
	Level        int
	Link         []string
	Maximum      *float64
	Minimum      *float64
	Path         string
	Pattern      string
	Required     bool
	Title        string
	Type         string
}

// Hijack the "example" field for custom documentation fields
func (m Doc) ApplyToSchema(schema *apiext.JSONSchemaProps) error {
	d := &Doc{}
	if schema.Example != nil {
		err := json.Unmarshal(schema.Example.Raw, d)
		if err != nil {
			return err
		}
	}
	err := mergo.Merge(d, m)
	if err != nil {
		return err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	schema.Example = &apiext.JSON{Raw: b}
	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func run() error {
	log.SetLevel(log.DebugLevel)

	cfg := &Config{}
	pflag.StringVar(&cfg.Directory, "dir", cfg.Directory, "directory with packages")
	pflag.StringVar(&cfg.Group, "group", cfg.Group, "which group to generate documentation for")
	pflag.StringVar(&cfg.Kind, "kind", cfg.Kind, "which kind to generate documentation for")
	pflag.StringVar(&cfg.ReferenceOutput, "reference-output", cfg.ReferenceOutput, "reference doc markdown output file")
	pflag.StringVar(&cfg.ExampleOutput, "example-output", cfg.ExampleOutput, "example yaml markdown output file")
	pflag.Parse()

	packages, err := loader.LoadRoots(cfg.Directory)
	if err != nil {
		return err
	}
	registry := &markers.Registry{}
	collector := &markers.Collector{
		Registry: registry,
	}

	err = crd_markers.Register(registry)
	if err != nil {
		return err
	}

	err = registry.Define("nais:doc", markers.DescribesField, Doc{})
	if err != nil {
		return fmt.Errorf("register marker: %w", err)
	}

	typechecker := &loader.TypeChecker{}
	pars := &crd.Parser{
		Collector: collector,
		Checker:   typechecker,
	}

	for _, pkg := range packages {
		pars.NeedPackage(pkg)
	}

	metav1Pkg := crd.FindMetav1(packages)
	if metav1Pkg == nil {
		return fmt.Errorf("no objects in the roots, since nothing imported metav1")
	}

	kubeKinds := crd.FindKubeKinds(pars, metav1Pkg)
	if len(kubeKinds) == 0 {
		return fmt.Errorf("no objects in the roots")
	}

	gk := schema.GroupKind{
		Group: cfg.Group,
		Kind:  cfg.Kind,
	}

	pars.NeedCRDFor(gk, nil)

	if len(pars.FlattenedSchemata) == 0 {
		return fmt.Errorf("schema generation failed; double check the syntax of doctags (+nais:* and +kubebuilder:*")
	}

	referenceOut := os.Stdout
	if len(cfg.ReferenceOutput) > 0 {
		referenceOut, err = os.OpenFile(cfg.ReferenceOutput, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	}
	refmw := &multiwriter{w: referenceOut}

	exampleOut := os.Stdout
	if len(cfg.ExampleOutput) > 0 {
		exampleOut, err = os.OpenFile(cfg.ExampleOutput, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	}
	exmw := &multiwriter{w: exampleOut}

	app := nais_io_v1alpha1.Application{}
	err = nais_io_v1alpha1.ApplyApplicationDefaults(&app)
	if err != nil {
		return err
	}
	data, err := json.Marshal(app.Spec)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &defaultApplication)
	if err != nil {
		return err
	}

	disclaimer := "<!--\n  This documentation was automatically generated by the liberator pipeline.\n  See https://github.com/nais/liberator/actions\n\n  DO NOT MAKE MANUAL CHANGES TO THIS FILE, THEY WILL BE OVERWRITTEN!\n-->\n\n"
	io.WriteString(refmw, "# Application spec reference\n\n")
	io.WriteString(refmw, disclaimer)

	io.WriteString(exmw, "# Application spec full example\n\n")
	io.WriteString(exmw, disclaimer)

	for _, schemata := range pars.FlattenedSchemata {
		WriteReferenceDoc(refmw, 1, "", "NAIS application", schemata.Properties["spec"], schemata.Properties["spec"])
		WriteExampleDoc(exmw, 1, "", "", schemata, schemata)
	}

	return refmw.Error()
}

type multiwriter struct {
	w   io.Writer
	err error
}

func (m *multiwriter) Write(p []byte) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	n, err := m.w.Write(p)
	if err != nil {
		m.err = err
	}
	return n, err
}

func (m *multiwriter) Error() error {
	return m.err
}

func linefmt(format string, args ...interface{}) string {
	format = fmt.Sprintf(format, args...)
	if len(format) == 0 {
		format = "_no value_"
	}
	format = strings.ReplaceAll(format, "``", "_no value_")
	return format + "<br />\n"
}

func floatfmt(f *float64) string {
	if f == nil {
		return "+Inf"
	}
	return strconv.FormatFloat(*f, 'f', 0, 64)
}

func writeList(w io.Writer, list []string) {
	sort.Strings(list)
	max := len(list) - 1
	for i, item := range list {
		if len(item) > 0 {
			io.WriteString(w, fmt.Sprintf("`%s`", item))
		} else {
			io.WriteString(w, fmt.Sprintf("_(empty string)_"))
		}
		if i != max {
			io.WriteString(w, ", ")
		}
	}
	io.WriteString(w, "<br />\n")
}

func (m ExtDoc) formatStraight(w io.Writer) {
	io.WriteString(w, fmt.Sprintf("%s %s", strings.Repeat("#", m.Level), strings.TrimLeft(m.Path, ".")))
	io.WriteString(w, "\n")
	if len(m.Description) > 0 {
		io.WriteString(w, m.Description)
		io.WriteString(w, "\n\n")
	}
	if len(m.Link) > 0 {
		io.WriteString(w, "Relevant information:\n\n")
		for _, link := range m.Link {
			io.WriteString(w, fmt.Sprintf("* [%s](%s)\n", link, link))
		}
		io.WriteString(w, "\n")
	}
	io.WriteString(w, linefmt("Type: `%s`", m.Type))
	io.WriteString(w, linefmt("Required: `%s`", strconv.FormatBool(m.Required)))
	if len(m.Default) > 0 {
		io.WriteString(w, linefmt("Default value: `%v`", m.Default))
	}
	if len(m.Availability) > 0 {
		io.WriteString(w, linefmt("Availability: %s", m.Availability))
	}
	if len(m.Pattern) > 0 {
		io.WriteString(w, linefmt("Pattern: `%s`", m.Pattern))
	}
	if m.Minimum != m.Maximum {
		min := floatfmt(m.Minimum)
		max := floatfmt(m.Maximum)
		switch {
		case m.Minimum == nil:
			io.WriteString(w, linefmt("Maximum value: `%s`", max))
		case m.Maximum == nil:
			io.WriteString(w, linefmt("Minimum value: `%s`", min))
		default:
			io.WriteString(w, linefmt("Value range: `%s`-`%s`", min, max))
		}
	}
	if len(m.Enum) > 0 {
		io.WriteString(w, "Allowed values: ")
		writeList(w, m.Enum)
	}
	io.WriteString(w, "\n")
}

func hasRequired(node apiext.JSONSchemaProps, key string) bool {
	for _, k := range node.Required {
		if k == key {
			return true
		}
	}

	if node.Items == nil {
		return false
	}

	for _, k := range node.Items.Schema.Required {
		if k == key {
			return true
		}
	}

	return false
}

func injectLinks(yml string) string {
	// FIXME
	return yml
}

func WriteExampleDoc(w io.Writer, level int, jsonpath string, key string, parent, node apiext.JSONSchemaProps) {
	app := nais_io_v1alpha1.ExampleApplicationForDocumentation()

	js, _ := json.Marshal(app)
	ym, _ := yaml2.JSONToYAML(js)

	rendered := injectLinks(string(ym))

	io.WriteString(w, "``` yaml\n")
	io.WriteString(w, rendered)
	io.WriteString(w, "```\n")

	return
}

func WriteReferenceDoc(w io.Writer, level int, jsonpath string, key string, parent, node apiext.JSONSchemaProps) {
	if jsonpath == ".metadata" || jsonpath == ".status" {
		return
	}

	if len(node.Enum) > 0 {
		node.Type = "enum"
	}

	entry := &ExtDoc{
		Description: strings.TrimSpace(node.Description),
		Level:       level,
		Maximum:     node.Maximum,
		Minimum:     node.Minimum,
		Path:        jsonpath,
		Pattern:     node.Pattern,
		Required:    hasRequired(parent, key),
		Title:       key,
		Type:        node.Type,
	}

	// Override children when encountering an array
	if node.Type == "array" {
		node.Properties = node.Items.Schema.Properties
		jsonpath += "[]"
	}

	defaultValue, err := getValueFromStruct(strings.Trim(jsonpath, "."), defaultApplication)
	if err == nil {
		entry.Default = fmt.Sprintf("%v", defaultValue)
	}

	if len(node.Enum) > 0 {
		entry.Enum = make([]string, 0, len(entry.Enum))
		for _, v := range node.Enum {
			s := ""
			err := json.Unmarshal(v.Raw, &s)
			if err != nil {
				s = string(v.Raw)
			}
			entry.Enum = append(entry.Enum, s)
		}
	}

	if node.Example != nil {
		d := &Doc{}
		err := json.Unmarshal(node.Example.Raw, d)
		if err == nil {
			entry.Availability = d.Availability
			entry.Link = d.Link
		} else {
			log.Errorf("unable to merge structs: %s", err)
		}
	}

	if len(jsonpath) > 0 {
		entry.formatStraight(w)

		example, err := getStructSubPath(strings.Trim(jsonpath, "."), exampleApplication)
		if err == nil {
			io.WriteString(w, "??? example\n")
			io.WriteString(w, "    ``` yaml\n")
			buf := bytes.NewBuffer(nil)
			enc := yaml.NewEncoder(buf)
			enc.SetIndent(2)
			enc.Encode(map[string]interface{}{"spec": example})
			scan := bufio.NewScanner(buf)
			for scan.Scan() {
				io.WriteString(w, "    "+scan.Text()+"\n")
			}
			io.WriteString(w, "    ```\n\n")
		}
	}

	if len(node.Properties) == 0 {
		return
	}

	keys := make([]string, 0)
	for k := range node.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		WriteReferenceDoc(w, level+1, jsonpath+"."+k, k, node, node.Properties[k])
	}
}

func getExampleApp() interface{} {
	app := nais_io_v1alpha1.ExampleApplicationForDocumentation()
	var marshaled interface{}
	data, err := json.Marshal(app.Spec)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(data, &marshaled)
	if err != nil {
		return nil
	}
	return marshaled
}

func getStructSubPath(keyWithDots string, object interface{}) (interface{}, error) {
	structure := make(map[string]interface{})
	var leaf interface{} = structure

	keySlice := strings.Split(keyWithDots, ".")
	v := reflect.ValueOf(object)

	resolve := func(v reflect.Value) reflect.Value {
		if v.Kind() == reflect.Ptr {
			return v.Elem()
		}
		return v
	}

	max := len(keySlice) - 1
	for i, key := range keySlice {
		key = strings.TrimRight(key, "[]")

		if len(key) == 0 {
			break
		}

		v = resolve(v)

		var added interface{}

		switch v.Kind() {
		case reflect.Map:
			drilldown := func() error {
				for _, k := range v.MapKeys() {
					if k.String() == key {
						v = v.MapIndex(k).Elem()
						return nil
					}
				}
				return fmt.Errorf("key not found")
			}

			err := drilldown()
			if err != nil {
				return nil, err
			}
		}

		v = resolve(v)

		switch {
		case v.Kind() == reflect.Slice:
			fallthrough
		case i == max:
			added = resolve(v).Interface()

		case v.Kind() == reflect.Map:
			added = make(map[string]interface{})
		}

		switch typedleaf := leaf.(type) {
		case map[string]interface{}:
			typedleaf[key] = added
		case []interface{}:
			typedleaf[0] = added
		}

		leaf = added
		if v.Kind() == reflect.Slice {
			break
		}
	}

	return structure, nil
}

func getValueFromStruct(keyWithDots string, object interface{}) (interface{}, error) {
	keySlice := strings.Split(keyWithDots, ".")
	v := reflect.ValueOf(object)

	for _, key := range keySlice {
		if len(key) == 0 {
			break
		}
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Map {
			return nil, fmt.Errorf("only accepts maps; got %T", v)
		}
		getKey := func() error {
			for _, k := range v.MapKeys() {
				if k.String() == key {
					v = v.MapIndex(k).Elem()
					return nil
				}
			}
			return fmt.Errorf("key not found")
		}
		err := getKey()
		if err != nil {
			return nil, err
		}
	}

	if !v.IsValid() {
		return nil, fmt.Errorf("no value")
	}

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
	case reflect.Int:
	case reflect.Bool:
	case reflect.Float64:
	default:
		return nil, fmt.Errorf("only scalar values supported")
	}

	return v.Interface(), nil
}

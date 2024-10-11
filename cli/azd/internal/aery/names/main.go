package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	//nolint:ST1001
	. "github.com/azure/azure-dev/cli/azd/internal/aerygen"
	"github.com/braydonk/yaml"
	flag "github.com/spf13/pflag"
)

var (
	token      = flag.String("token", "", "GitHub token")
	emitSource = flag.Bool("emit-source", false, "Emit the fetched source documents to disk.")
	// /Users/weilim/repos/azure-dev/cli/azd/resources/aery-gen/names.yaml
	outFile = flag.String("out-file", "names.yaml", "Output file")
)

func run() error {
	// map of resource type to resource
	resources := map[string][]ResourceKind{}
	err := addAbbreviations(resources)
	if err != nil {
		return fmt.Errorf("adding abbreviations: %w", err)
	}

	err = addNamingRules(resources)
	if err != nil {
		return fmt.Errorf("adding naming rules: %w", err)
	}

	if app, ok := resources["Microsoft.App/containerApps"]; ok {
		// we don't want abbreviation for container apps
		for _, kind := range app {
			kind.Abbreviation = ""
		}
	}

	kindCount := 0
	resourceTypes := slices.Collect(maps.Keys(resources))
	slices.Sort(resourceTypes)
	for _, resType := range resourceTypes {
		kindCount += len(resources[resType])
		for _, kind := range resources[resType] {
			if kind.Kind != "" {
				fmt.Printf("- %s:%s\n", resType, kind.Kind)
			} else if kind.CustomKind.Value != "" {
				fmt.Printf("- %s:%s\n", resType, kind.CustomKind.Value)
			} else {
				fmt.Printf("- %s\n", resType)
			}
		}
	}

	fmt.Printf("total resource types: %d\n", len(resources))
	fmt.Printf("total resource kinds: %d\n", kindCount)

	buf := bytes.Buffer{}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err = enc.Encode(AzureNames{Types: resources})
	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}

	return os.WriteFile(*outFile, buf.Bytes(), 0644)
}

var resourceTypeRegex = regexp.MustCompile(`\s*` +
	"`" + `([\w+\.\/]+)` + "`" +
	`(?:\s*\(kind:\s*` + "`" + `([\w\.]+)` + "`" + `\))?`)

var unsupportedTypes = map[string]struct{}{
	// TODO: figure out how kinds work in this model
	"Microsoft.HDInsight/clusters": {},
}

func addAbbreviations(resources map[string][]ResourceKind) error {
	content, err := fetchGithub(
		*token,
		"MicrosoftDocs/cloud-adoption-framework",
		"docs/ready/azure-best-practices/resource-abbreviations.md")
	if err != nil {
		return err
	}

	markdownContent := string(content)
	lines := strings.Split(markdownContent, "\n")
	inTable := false

	for _, line := range lines {
		if strings.Contains(line, "|--|--|--|") { // header line, 3 columns
			inTable = true
			continue
		}

		if inTable {
			if !strings.Contains(line, "|") {
				inTable = false
				continue
			}

			// split example:
			// | AI Search | `Microsoft.Search/searchServices` | `srch` |
			l := strings.TrimSpace(line)
			elements := strings.Split(l[1:len(l)-1], "|")
			if len(elements) != 3 {
				return fmt.Errorf("parsing '%s': invalid number of columns", line)
			}

			res := ResourceKind{}
			res.Name = strings.TrimSpace(elements[0])

			if strings.HasPrefix(res.Name, "Azure Cosmos DB for") {
				// Skips:
				// - Azure Cosmos DB for Apache Cassandra account
				// - Azure Cosmos DB for MongoDB account
				// - Azure Cosmos DB for NoSQL account
				// - Azure Cosmos DB for Table account
				// - Azure Cosmos DB for Apache Gremlin account
				continue
			}

			if strings.HasPrefix(res.Name, "ExpressRoute") {
				// Skips:
				// | ExpressRoute circuit | `Microsoft.Network/expressRouteCircuits` | `erc` |
				// | ExpressRoute gateway | `Microsoft.Network/virtualNetworkGateways` | `ergw` |
				// these networks have other abbreviations defined
				continue
			}

			if strings.HasPrefix(res.Name, "VM storage account") {
				// Skips:
				// | VM storage account | `Microsoft.Storage/storageAccounts` | `stvm` |
				continue
			}

			if strings.HasPrefix(res.Name, "Hosting environment") {
				// Skips:
				// | Hosting environment | `Microsoft.Web/hostingEnvironments` | `host` |
				continue
			}

			resType, kind := parseResourceType(elements[1])
			if _, ok := unsupportedTypes[resType]; ok {
				continue
			}
			res.Kind = kind

			abbreviation := strings.TrimSpace(elements[2])
			res.Abbreviation = abbreviation[1 : len(abbreviation)-1]

			if res.Abbreviation == "func" {
				res.Kind = "functionapp"
			} else if res.Abbreviation == "app" {
				res.Kind = "app"
			}

			resources[resType] = append(resources[resType], res)
		}
	}

	return nil
}

func parseResourceType(markdownString string) (resType string, resKind string) {
	resourceTypeStrings := resourceTypeRegex.FindStringSubmatch(markdownString)
	switch len(resourceTypeStrings) {
	case 3:
		return resourceTypeStrings[1], resourceTypeStrings[2]
	case 2:
		return resourceTypeStrings[1], ""
	default:
		return "", ""
	}
}

func addNamingRules(resources map[string][]ResourceKind) error {
	content, err := fetchGithub(
		*token,
		"mspnp/AzureNamingTool",
		"src/repository/resourcetypes.json")
	if err != nil {
		return err
	}

	var namingToolResourceTypes []namingToolResourceTypes
	err = json.Unmarshal(content, &namingToolResourceTypes)
	if err != nil {
		return fmt.Errorf("unmarshaling: %w", err)
	}

	for _, r := range namingToolResourceTypes {
		if r.ShortName == "" { // handle potential casing inconsistency
			r.ShortName = r.ShortNameOtherCase
		}

		if strings.Count(r.Resource, "/") > 1 {
			// skip child resources for now
			continue
		}

		resourceType := fmt.Sprintf("Microsoft.%s", r.Resource)
		switch resourceType { // handle incorrect namings
		case "Microsoft.Resources/resourcegroups":
			resourceType = "Microsoft.Resources/resourceGroups"
		case "Microsoft.Web/serverfarms":
			resourceType = "Microsoft.Web/serverFarms"
		case "Microsoft.SignalRService/signalR":
			resourceType = "Microsoft.SignalRService/SignalR"
		}

		// map of resource type->kind to override the abbreviation
		overrideList := map[string]string{
			"Microsoft.Cdn/profiles":          "", // markdown says 'afd'
			"Microsoft.Compute/disks":         "", // markdown says 'osdisk' and 'disk' depending on OS type
			"Microsoft.Network/frontDoors":    "", // choose 'fd' over 'afd'
			"Microsoft.ServiceBus/namespaces": "", // choose 'sb' over 'sbns'
		}

		// map of resource type->kind to override the abbreviation
		ignoreList := map[string]string{
			"Microsoft.DBforMariaDB/servers":           "", // choose 'maria' over 'mdbsv'
			"Microsoft.DBforMariaDB/servers/databases": "", // choose 'mariadb' over 'mdbdb'
			"Microsoft.Maps/accounts":                  "", // choose 'map' over 'macc'
			"Microsoft.Network/firewallPolicies":       "", // choose 'afwp' over 'waf'
			"Microsoft.Resources/templateSpecs":        "", // choose 'ts' over 'tspec'
		}

		var upsert *ResourceKind
		new := true

		if _, ok := unsupportedTypes[resourceType]; ok {
			continue
		}

		kind := ""
		if resourceType == "Microsoft.Web/sites" {
			switch r.Property {
			case "Function App":
				kind = "functionapp"
			case "Web App":
				kind = "app"
			case "Static Web App":
				// deprecated: Microsoft.Web/staticSites
				continue
			case "Azure Static Web Apps":
				// deprecated: Microsoft.Web/staticSites
				continue
			case "App Service Environment":
				// Microsoft.Web/hostingEnvironments
				continue
			}
		}

		if resKinds, exists := resources[resourceType]; exists {
			for i := range resKinds {
				if resKinds[i].Kind == kind {
					new = false
					upsert = &resKinds[i]
					break
				}
			}

			if upsert == nil {
				upsert = &ResourceKind{
					Kind: kind,
					// Name:         don't have a name,
					Abbreviation: r.ShortName,
				}
			} else if kind, ok := overrideList[resourceType]; ok && kind == upsert.Kind {
				upsert.Abbreviation = r.ShortName
			} else if kind, ok := ignoreList[resourceType]; ok && kind == upsert.Kind {
				// do nothing
			} else if strings.ContainsAny(upsert.Abbreviation, "<*") { // automatic override -- the markdown has metadata in it
				upsert.Abbreviation = r.ShortName
			} else if r.ShortName != upsert.Abbreviation {
				if resourceType == "Microsoft.Sql/servers" && r.Property == "Azure SQL Data Warehouse" {
					continue
				}

				if resourceType == "Microsoft.Storage/storageAccounts" && r.Property == "VM Storage Account" {
					continue
				}

				if resourceType == "Microsoft.Network/loadBalancers" {
					r.ShortName = "lb"
				} else {
					return fmt.Errorf("resource type '%s' already exists with different abbreviation: '%s' vs '%s'",
						resourceType, upsert.Abbreviation, r.ShortName)
				}
			} // else r.Short == upsert.Abbreviation, nothing to do
		} else {
			upsert = &ResourceKind{
				Kind: kind,
				// Name:         don't have a name,
				Abbreviation: r.ShortName,
			}
		}

		rule := NamingRules{}

		if strings.Contains(r.Regx, "(?!") { // Perl-style negative lookahead not supported
			// do nothing currently
		} else {
			regex := r.Regx
			if regex == `^[A-Za-z0-9-_\.~]{1,1024}$` {
				regex = `^[A-Za-z0-9-_\.~]{1,1000}([A-Za-z0-9-_\.~]{0,24})$`
			}

			// first, validate if the regex is valid
			nameRegex, err := regexp.Compile(regex)
			if err != nil {
				return fmt.Errorf("regexp parsing '%s': compiling regex: %w", r.Regx, err)
			}

			// then, validate if the regex accepts hyphens
			if nameRegex.MatchString("foo-bar") {
				rule.WordSeparator = "-"
			}
			rule.Regex = regex
		}

		if r.LengthMin != "" {
			minLength, err := strconv.Atoi(r.LengthMin)
			if err != nil {
				return fmt.Errorf("parsing min length: %w", err)
			}

			rule.MinLength = minLength
		}

		if r.LengthMax != "" {
			maxLength, err := strconv.Atoi(r.LengthMax)
			if err != nil {
				return fmt.Errorf("parsing max length: %w", err)
			}
			rule.MaxLength = maxLength
		}

		scope := r.Scope
		// Current allowed scopes:
		// - container
		// - global
		// - region
		// - resource
		// - resource group
		// - resource group & region
		// - scope of assignment
		// - scope of definition
		// - storage account
		// - subscription
		// - tenant
		// - workspace
		switch r.Scope {
		case "resource group":
			scope = "group"
		}
		rule.UniquenessScope = scope

		messages := Messages{}
		messages.OnFailure = r.InvalidText
		messages.OnSuccess = r.ValidText
		rule.Messages = messages

		restrictedChars := RestrictedChars{}
		restrictedChars.Global = r.InvalidCharacters
		restrictedChars.Prefix = r.InvalidCharactersStart
		restrictedChars.Suffix = r.InvalidCharactersEnd
		restrictedChars.Consecutive = r.InvalidCharactersConsecutive
		rule.RestrictedChars = restrictedChars

		// derived rule from restricted characters
		if strings.ContainsAny(restrictedChars.Global, "-") {
			rule.WordSeparator = ""
		}

		upsert.NamingRules = rule

		if new {
			resources[resourceType] = append(resources[resourceType], *upsert)
		}
	}

	return nil
}

// "id": 3,
// "resource": "ApiManagement/service/apis",
// "optional": "UnitDept",
// "exclude": "Org,Function",
// "property": "",
// "ShortName": "apis",
// "scope": "service",
// "lengthMin": "1",
// "lengthMax": "256",
// "validText": "",
// "invalidText": "Asterisk, number sign, ampersand, plus, colon, angle brackets, question mark.",
// "invalidCharacters": "*#&+:<>?",
// "invalidCharactersStart": "",
// "invalidCharactersEnd": "",
// "invalidCharactersConsecutive": "",
// "regx": "^[^\\*#&\\+:<>\\?]{1,256}$",
// "staticValues": "

type namingToolResourceTypes struct {
	ID                           int    `json:"id"`
	Resource                     string `json:"resource"`
	Optional                     string `json:"optional"`
	Exclude                      string `json:"exclude"`
	Property                     string `json:"property"`
	ShortName                    string `json:"ShortName"`
	ShortNameOtherCase           string `json:"shortName"`
	Scope                        string `json:"scope"`
	LengthMin                    string `json:"lengthMin"`
	LengthMax                    string `json:"lengthMax"`
	ValidText                    string `json:"validText"`
	InvalidText                  string `json:"invalidText"`
	InvalidCharacters            string `json:"invalidCharacters"`
	InvalidCharactersStart       string `json:"invalidCharactersStart"`
	InvalidCharactersEnd         string `json:"invalidCharactersEnd"`
	InvalidCharactersConsecutive string `json:"invalidCharactersConsecutive"`
	Regx                         string `json:"regx"`
	StaticValues                 string `json:"staticValues"`
}

func main() {
	flag.Parse()
	if token == nil || *token == "" {
		*token = os.Getenv("GITHUB_TOKEN")
	}

	if token == nil || *token == "" {
		fmt.Fprintln(os.Stderr, "error: missing GitHub token")
		flag.Usage()
		os.Exit(1)
	}

	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

// path example: specification/cognitiveservices/resource-manager/Microsoft.CognitiveServices/stable/2023-05-01/cognitiveservices.json
// token example: GITHUB_TOKEN
func fetchGithub(
	token string,
	repo string,
	path string) ([]byte, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.raw")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	if emitSource != nil && *emitSource {
		lastSep := strings.LastIndex(path, "/")
		fileName := path[lastSep+1:]
		err = os.WriteFile(fileName, content, 0644)
		if err != nil {
			return nil, fmt.Errorf("writing file: %w", err)
		}
	}

	_ = resp.Body.Close()
	return content, nil
}

package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type ProductionFlow struct {
	Resource string `json:"resource"`
	Amount   string `json:"amount"`
	Unit     string `json:"unit"`
}
type ProductionRule struct {
	Inputs   []ProductionFlow `json:"inputs"`
	Outputs  []ProductionFlow `json:"outputs"`
	Batches  int              `json:"batches"`
	Priority int              `json:"priority"`
}

func productionRule(r Record) (*ProductionRule, error) {
	value, ok := r.Properties["_production"]
	if !ok {
		return nil, nil
	}
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	var rule ProductionRule
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&rule); e != nil {
		return nil, fmt.Errorf("invalid production recipe: %w", e)
	}
	return &rule, nil
}

func validateProduction(r Record, records map[string]Record) error {
	rule, e := productionRule(r)
	if e != nil {
		return e
	}
	if rule == nil {
		return nil
	}
	if r.Kind != "entity" || r.Properties["_domain"] != "recipe" {
		return fmt.Errorf("production rules belong to recipe entities")
	}
	if rule.Batches < 0 || rule.Batches > 1000000 || rule.Priority < 0 || rule.Priority > 10000 {
		return fmt.Errorf("recipe batches must be 0–1000000 and priority 0–10000")
	}
	if len(rule.Inputs) > 20 || len(rule.Outputs) == 0 || len(rule.Outputs) > 20 {
		return fmt.Errorf("a recipe needs 1–20 outputs and at most 20 inputs")
	}
	for _, flows := range [][]ProductionFlow{rule.Inputs, rule.Outputs} {
		seen := map[string]bool{}
		for _, flow := range flows {
			resource, ok := records[flow.Resource]
			if !ok || resource.Kind != "entity" || resource.Properties["_domain"] != "resource" || seen[flow.Resource] {
				return fmt.Errorf("recipe needs distinct existing resources on each side")
			}
			seen[flow.Resource] = true
			amount, e := decimal(flow.Amount)
			if e != nil || amount.Sign() <= 0 {
				return fmt.Errorf("recipe amounts must be positive decimals")
			}
			if strings.TrimSpace(flow.Unit) == "" || len(flow.Unit) > 100 {
				return fmt.Errorf("recipe quantities need an explicit unit of at most 100 bytes")
			}
		}
	}
	return nil
}

func exactDecimal(value *big.Rat) (string, error) {
	denominator := new(big.Int).Set(value.Denom())
	two, five := 0, 0
	for _, factor := range []int64{2, 5} {
		divisor := big.NewInt(factor)
		for denominator.Cmp(big.NewInt(1)) > 0 {
			quotient, remainder := new(big.Int), new(big.Int)
			quotient.QuoRem(denominator, divisor, remainder)
			if remainder.Sign() != 0 {
				break
			}
			denominator = quotient
			if factor == 2 {
				two++
			} else {
				five++
			}
		}
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("result cannot be represented as an exact decimal")
	}
	scale := max(two, five)
	result := value.FloatString(scale)
	if scale > 0 {
		result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	}
	if _, e := decimal(result); e != nil {
		return "", e
	}
	return result, nil
}

func productionExperiment(c ExperimentRequest, st State, selected []Record, out *ExperimentResult) error {
	periods, e := strconv.Atoi(c.Values["periods"])
	if e != nil || periods < 1 || periods > 100 {
		return fmt.Errorf("choose 1–100 whole production periods")
	}
	type recipe struct {
		record    Record
		rule      *ProductionRule
		completed int
		limited   map[string]bool
	}
	recipes := []*recipe{}
	stock := map[string]*big.Rat{}
	original := map[string]*big.Rat{}
	units := map[string]string{}
	for _, r := range selected {
		rule, e := productionRule(r)
		if e != nil {
			return e
		}
		if rule == nil {
			continue
		}
		if e = validateProduction(r, st.Records); e != nil {
			return e
		}
		recipes = append(recipes, &recipe{record: r, rule: rule, limited: map[string]bool{}})
		for _, flow := range append(append([]ProductionFlow{}, rule.Inputs...), rule.Outputs...) {
			resource := st.Records[flow.Resource]
			var quantity Quantity
			raw, _ := json.Marshal(resource.Properties["quantity"])
			if json.Unmarshal(raw, &quantity) != nil || quantity.Mode != "exact" || quantity.Unit == "" || quantity.Unit != flow.Unit {
				return fmt.Errorf("%s needs an exact inventory in %s; unknown, ranged or mismatched units are not inferred", resource.Name, flow.Unit)
			}
			value, e := decimal(quantity.Value)
			if e != nil || value.Sign() < 0 {
				return fmt.Errorf("%s needs a nonnegative inventory", resource.Name)
			}
			if stock[resource.ID] == nil {
				stock[resource.ID] = value
				original[resource.ID] = new(big.Rat).Set(value)
				units[resource.ID] = quantity.Unit
			}
		}
	}
	if len(recipes) == 0 {
		return fmt.Errorf("select at least one recipe with saved production rules")
	}
	if len(stock) > 500 {
		return fmt.Errorf("this experiment touches more than 500 resources; select a smaller network")
	}
	sort.Slice(recipes, func(i, j int) bool {
		a, b := recipes[i], recipes[j]
		if a.rule.Priority != b.rule.Priority {
			return a.rule.Priority < b.rule.Priority
		}
		if a.record.Name != b.record.Name {
			return a.record.Name < b.record.Name
		}
		return a.record.ID < b.record.ID
	})
	for period := 0; period < periods; period++ {
		for _, recipe := range recipes {
			batches := recipe.rule.Batches
			bounds := map[string]int{}
			for _, flow := range recipe.rule.Inputs {
				amount, _ := decimal(flow.Amount)
				possible := new(big.Rat).Quo(stock[flow.Resource], amount)
				whole := new(big.Int).Quo(possible.Num(), possible.Denom())
				if whole.Cmp(big.NewInt(int64(recipe.rule.Batches))) < 0 {
					bounds[flow.Resource] = int(whole.Int64())
					batches = min(batches, int(whole.Int64()))
				}
			}
			for id, bound := range bounds {
				if bound == batches {
					recipe.limited[st.Records[id].Name] = true
				}
			}
			recipe.completed += batches
			for _, flow := range recipe.rule.Inputs {
				amount, _ := decimal(flow.Amount)
				used := new(big.Rat).Mul(amount, new(big.Rat).SetInt64(int64(batches)))
				stock[flow.Resource].Sub(stock[flow.Resource], used)
			}
			for _, flow := range recipe.rule.Outputs {
				amount, _ := decimal(flow.Amount)
				made := new(big.Rat).Mul(amount, new(big.Rat).SetInt64(int64(batches)))
				stock[flow.Resource].Add(stock[flow.Resource], made)
			}
		}
	}
	for _, recipe := range recipes {
		limited := []string{}
		for name := range recipe.limited {
			limited = append(limited, name)
		}
		sort.Strings(limited)
		out.Rows = append(out.Rows, map[string]string{"recipe": recipe.record.Name, "requested batches": strconv.Itoa(recipe.rule.Batches * periods), "completed batches": strconv.Itoa(recipe.completed), "limited by": strings.Join(limited, ", ")})
	}
	ids := []string{}
	for id := range stock {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := st.Records[id]
		after, e := exactDecimal(stock[id])
		if e != nil {
			return fmt.Errorf("%s result exceeds exact quantity limits: %w", r.Name, e)
		}
		before, _ := exactDecimal(original[id])
		out.Rows = append(out.Rows, map[string]string{"resource": r.Name, "before": before, "after": after, "unit": units[id]})
		if stock[id].Cmp(original[id]) == 0 {
			continue
		}
		r = cloneProperties(r)
		quantity := map[string]any{}
		raw, _ := json.Marshal(r.Properties["quantity"])
		json.Unmarshal(raw, &quantity)
		quantity["value"] = after
		r.Properties["quantity"] = quantity
		r.Properties["productionSnapshot"] = st.Age.Snapshot
		r.Properties["productionPeriods"] = periods
		r.Properties["productionRecipes"] = append([]string{}, c.IDs...)
		out.Proposals = append(out.Proposals, r)
	}
	out.AllOrNothing = true
	out.Warnings = append(out.Warnings, "Whole batches run in ascending authored priority, then by name. Each recipe's outputs become available to later recipes in that period; a return through a cycle waits for the next period.", "Inputs, outputs and starting inventories must use matching explicit units. No conversion, labour, transport, storage limits, population effects or political consequences are inferred. Inputless recipes assume externally available sources.", "Resource balances are linked and are accepted together as one change.")
	return nil
}

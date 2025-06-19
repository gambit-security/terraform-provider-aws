package tools

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
)

func ReadResource(aws_config aws.Config, resource_type string, id string) (any, error) {
	conns.SetResourceBinderAWSConfig(aws_config)

	resourceFunc, ok := resources[resource_type]
	if !ok {
		return nil, fmt.Errorf("resource type %s not found", resource_type)
	}
	resource := resourceFunc()
	data := resource.Data(nil)
	data.SetId(id)

	ctx := context.Background()

	p, err := provider.New(ctx)
	if err != nil {
		return nil, err
	}
	meta := p.Meta().(*conns.AWSClient)

	resource.ReadWithoutTimeout(ctx, data, meta)

	attr_value, err := data.State().AttrsAsObjectValue(resource.CoreConfigSchema().ImpliedType())
	if err != nil {
		return nil, err
	}
	attr := convertCtyValue(attr_value)

	return attr, nil
}

func convertCtyMap(ctyMap map[string]*cty.Value) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, valPtr := range ctyMap {
		if valPtr == nil || valPtr.IsNull() {
			result[key] = []string{}
			continue
		}

		val := *valPtr // Dereference pointer

		switch {
		case val.Type().IsPrimitiveType():
			// Convert primitive types
			if val.Type().Equals(cty.String) {
				result[key] = val.AsString()
			} else if val.Type().Equals(cty.Number) {
				floatVal, _ := val.AsBigFloat().Float64()
				result[key] = floatVal
			} else if val.Type().Equals(cty.Bool) {
				result[key] = val.True()
			}
		case val.Type().IsListType() || val.Type().IsTupleType() || val.Type().IsSetType():
			// Convert list/tuple to slice
			listVals := val.AsValueSlice()
			listResult := make([]interface{}, len(listVals))
			for i, v := range listVals {
				listResult[i] = convertCtyValue(v)
			}
			result[key] = listResult
		case val.Type().IsMapType() || val.Type().IsObjectType():
			// Convert nested maps/objects
			mapVals := val.AsValueMap()
			convertedMap, err := convertCtyMapPtr(mapVals)
			if err != nil {
				return nil, err
			}
			result[key] = convertedMap
		default:
			return nil, fmt.Errorf("unsupported cty.Value type: %s", val.Type().FriendlyName())
		}
	}

	return result, nil
}

// Convert *cty.Value to interface{} (helper function)
func convertCtyValue(val cty.Value) interface{} {
	if val.IsNull() {
		return []string{}
	}

	if val.Type().IsPrimitiveType() {
		if val.Type().Equals(cty.String) {
			return val.AsString()
		} else if val.Type().Equals(cty.Number) {
			floatVal, _ := val.AsBigFloat().Float64()
			return floatVal
		} else if val.Type().Equals(cty.Bool) {
			return val.True()
		}
	} else if val.Type().IsListType() || val.Type().IsTupleType() {
		listVals := val.AsValueSlice()
		listResult := make([]interface{}, len(listVals))
		for i, v := range listVals {
			listResult[i] = convertCtyValue(v)
		}
		return listResult
	} else if val.Type().IsMapType() || val.Type().IsObjectType() {
		mapVals := val.AsValueMap()
		convertedMap, _ := convertCtyMapPtr(mapVals)
		return convertedMap
	}
	return nil
}

// Convert map[string]cty.Value to map[string]interface{}
func convertCtyMapPtr(ctyMap map[string]cty.Value) (map[string]interface{}, error) {
	ptrMap := make(map[string]*cty.Value)
	for k, v := range ctyMap {
		val := v
		ptrMap[k] = &val
	}
	return convertCtyMap(ptrMap)
}

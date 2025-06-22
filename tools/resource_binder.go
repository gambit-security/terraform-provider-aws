package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	arnaws "github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/provider"
)

func ReadResource(awsConfig aws.Config, resource_type string, arn string) (any, error) {
	ctx := context.Background()

	creds, err := awsConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	rawData := map[string]interface{}{
		"region":     awsConfig.Region,
		"access_key": creds.AccessKeyID,
		"secret_key": creds.SecretAccessKey,
	}

	// SessionToken is optional, only needed for assumed role or temporary credentials
	if creds.SessionToken != "" {
		rawData["token"] = creds.SessionToken
	}

	providerConfig := terraform.NewResourceConfigRaw(rawData)

	p, err := provider.New(ctx)
	if err != nil {
		return nil, err
	}

	// Call ConfigureContextFunc to initialize the provider
	diags := p.Configure(ctx, providerConfig)
	if diags.HasError() {
		return nil, fmt.Errorf("provider configuration failed: %w", diags)
	}

	resource, ok := p.ResourcesMap[resource_type]
	if !ok {
		return nil, fmt.Errorf("resource type %s unsupported", resource_type)
	}

	data := resource.Data(nil)

	// Normalize arn to id
	var id string
	if strings.HasPrefix(arn, "arn:") {
		arnObject, err := arnaws.Parse(arn)
		if err != nil {
			return nil, err
		}
		id = arnObject.Resource
		if strings.Contains(arn, "/") {
			id = strings.Split(arn, "/")[1] // Remove the first part of the ARN (e.g., "aws:iam::123456789012:role/ExampleRole" -> "ExampleRole")
		}
	}

	data.SetId(id)

	meta := p.Meta().(*conns.AWSClient)

	if data == nil && meta == nil {
		return nil, fmt.Errorf("failed to read resource: data or meta is nil")
	}

	diags = resource.ReadWithoutTimeout(ctx, data, meta)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to read resource %s with ID %s: %v", resource_type, arn, diags)
	}

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

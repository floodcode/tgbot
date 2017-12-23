package tgbot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var httpClient = &http.Client{}

func sendResuest(method string, apiKey string, paramsObject interface{}, t interface{}) error {
	var requestBytes bytes.Buffer
	var writer = multipart.NewWriter(&requestBytes)
	var withParameters = false

	var addFileParameter = func(key string, value []byte, filename string) {
		writer, _ := writer.CreateFormFile(key, filename)
		writer.Write(value)
		withParameters = true
	}

	var addStringParameter = func(key string, value string) {
		writer, _ := writer.CreateFormField(key)
		writer.Write([]byte(value))
		withParameters = true
	}

	parameters, err := extractParams(paramsObject)
	for key, value := range parameters {
		if param, ok := value.(string); ok {
			addStringParameter(key, param)
		} else if param, ok := value.(*InputFile); ok {
			fileData := param.getData()
			if stringData, ok := fileData.(string); ok {
				addStringParameter(key, stringData)
			} else if bytesData, ok := fileData.([]byte); ok {
				addFileParameter(key, bytesData, param.getFilename())
			}
		} else if param, ok := value.([]byte); ok {
			addFileParameter(key, param, "file")
		}
	}

	writer.Close()

	request, err := http.NewRequest("POST", "https://api.telegram.org/bot"+apiKey+"/"+method, &requestBytes)
	if err != nil {
		return errors.New("unable to create request with given parameters")
	}

	if withParameters {
		request.Header.Add("Content-Type", writer.FormDataContentType())
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return errors.New("unable to execute request with given parameters")
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.New("unable to read response body")
	}

	var apiResponse = apiResponse{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return errors.New("unable to unserialize response from server")
	}

	if apiResponse.Ok != true {
		return errors.New(apiResponse.Description)
	}

	err = json.Unmarshal(apiResponse.Result, &t)
	if err != nil {
		return errors.New("unable to unserialize API result")
	}

	return nil
}

func extractParams(paramsObject interface{}) (map[string]interface{}, error) {
	var result = map[string]interface{}{}

	if paramsObject == nil {
		return result, nil
	}

	reflectType := reflect.TypeOf(paramsObject)
	reflectValue := reflect.ValueOf(paramsObject)

	for i := 0; i < reflectType.NumField(); i++ {
		field := reflectType.Field(i)
		option := field.Tag.Get("option")

		var extractedValue interface{}
		switch v := reflectValue.FieldByName(field.Name).Interface().(type) {
		case bool:
			if v {
				extractedValue = "true"
			}
		case int:
			if v != 0 {
				extractedValue = strconv.FormatInt(int64(v), 10)
			}
		case int64:
			if v != 0 {
				extractedValue = strconv.FormatInt(v, 10)
			}
		case float64:
			if v != 0 {
				extractedValue = strconv.FormatFloat(v, 'f', 6, 64)
			}
		case string:
			if len(v) > 0 {
				extractedValue = v
			}
		case *InputFile:
			if v != nil {
				extractedValue = v
			}
		case stringConfig:
			if !reflect.ValueOf(v).IsNil() {
				extractedValue = v.getString()
			}
		case []InlineQueryResult:
			serializedQueryResults := []string{}
			for _, queryResult := range v {
				queryResultJSON := buildInlineQueryResult(queryResult, queryResult.getType())
				serializedQueryResults = append(serializedQueryResults, queryResultJSON)
			}
			extractedValue = "[" + strings.Join(serializedQueryResults, ",") + "]"
		}

		if extractedValue != nil {
			result[option] = extractedValue
		}
	}

	return result, nil
}

func buildInlineQueryResult(queryResult InlineQueryResult, resultType string) string {
	queryResultMap := map[string]interface{}{"type": resultType}
	reflectType := reflect.TypeOf(queryResult)
	reflectValue := reflect.ValueOf(queryResult)

	for i := 0; i < reflectType.NumField(); i++ {
		field := reflectType.Field(i)
		if jsonAttribute, ok := field.Tag.Lookup("json"); ok {
			val := reflectValue.FieldByName(field.Name).Interface()
			if val != reflect.Zero(reflect.TypeOf(val)).Interface() {
				queryResultMap[jsonAttribute] = val
			}
		}
	}

	queryResultJSON, _ := json.Marshal(queryResultMap)
	return string(queryResultJSON)
}

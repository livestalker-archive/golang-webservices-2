package week1

import "net/http"
import "encoding/json"
import "context"
import "reflect"
import "fmt"
import "strings"
import "strconv"

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		h.handlerProfile(w, r)
	case "/user/create":
		h.handlerCreate(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unknown method"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
		return
	}
}
func (h *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
	// 3. заполнение структуры params
	var vErr *ApiError
	valLogin, vErr := FillValue("Login", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	params := ProfileParams{
		Login: valLogin.(string),
	}
	// 4. валидирование параметров
	valErr := ValidateProfileParams(params)
	if valErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": valErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(valErr.HTTPStatus)
		w.Write(body)
		return
	}
	ctx := context.Background()
	answer, err := h.Profile(ctx, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": err.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(body)
		return
	}
	res := map[string]interface{}{
		"error":    "",
		"response": answer,
	}
	body, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
func (h *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	// 1. проверка авторизации
	if h, ok := r.Header["X-Auth"]; ok {
		if h[0] != "100500" {
			w.Header().Set("Content-Type", "application/json")
			res := map[string]string{"error": "unauthorized"}
			body, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write(body)
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unauthorized"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write(body)
		return
	}
	// 2. проверки метода (GET/POST)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "bad method"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(body)
		return
	}
	// 3. заполнение структуры params
	var vErr *ApiError
	valLogin, vErr := FillValue("Login", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valName, vErr := FillValue("full_name", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valStatus, vErr := FillValue("Status", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valAge, vErr := FillValue("Age", "int", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	params := CreateParams{
		Login:  valLogin.(string),
		Name:   valName.(string),
		Status: valStatus.(string),
		Age:    valAge.(int),
	}
	// 4. валидирование параметров
	valErr := ValidateCreateParams(params)
	if valErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": valErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(valErr.HTTPStatus)
		w.Write(body)
		return
	}
	ctx := context.Background()
	answer, err := h.Create(ctx, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": err.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(body)
		return
	}
	res := map[string]interface{}{
		"error":    "",
		"response": answer,
	}
	body, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
func (h *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		h.handlerCreate(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unknown method"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
		return
	}
}
func (h *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	// 1. проверка авторизации
	if h, ok := r.Header["X-Auth"]; ok {
		if h[0] != "100500" {
			w.Header().Set("Content-Type", "application/json")
			res := map[string]string{"error": "unauthorized"}
			body, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write(body)
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unauthorized"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write(body)
		return
	}
	// 2. проверки метода (GET/POST)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "bad method"}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(body)
		return
	}
	// 3. заполнение структуры params
	var vErr *ApiError
	valUsername, vErr := FillValue("Username", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valName, vErr := FillValue("account_name", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valClass, vErr := FillValue("Class", "string", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	valLevel, vErr := FillValue("Level", "int", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	params := OtherCreateParams{
		Username: valUsername.(string),
		Name:     valName.(string),
		Class:    valClass.(string),
		Level:    valLevel.(int),
	}
	// 4. валидирование параметров
	valErr := ValidateOtherCreateParams(params)
	if valErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": valErr.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(valErr.HTTPStatus)
		w.Write(body)
		return
	}
	ctx := context.Background()
	answer, err := h.Create(ctx, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": err.Error()}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(body)
		return
	}
	res := map[string]interface{}{
		"error":    "",
		"response": answer,
	}
	body, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
func FillValue(n, t string, r *http.Request) (interface{}, *ApiError) {
	n = strings.ToLower(n)
	val := r.FormValue(n)
	if t == "int" {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, &ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        fmt.Errorf("%s must be %s", n, t),
			}
		}
		return res, nil
	}
	return val, nil
}

func ValidateCreateParams(param CreateParams) *ApiError {
	var e reflect.Value
	// validate Login field
	// validate required status
	e = reflect.ValueOf(param).FieldByName("Login")
	if reflect.Zero(e.Type()).Interface() == e.Interface() {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must me not empty", strings.ToLower("Login")),
		}
	}
	// validate min value
	e = reflect.ValueOf(param).FieldByName("Login")
	if len(e.Interface().(string)) < 10 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s len must be >= %d", strings.ToLower("Login"), 10),
		}
	}
	// validate Name field
	// validate Status field
	// validate enum value
	enumVal := strings.Split("user|moderator|admin", "|")
	e = reflect.ValueOf(param).FieldByName("Status")
	var findVal bool
	for _, el := range enumVal {
		if el == e.Interface().(string) {
			findVal = true
			break
		}
	}
	if !findVal && reflect.Zero(e.Type()).Interface() == e.Interface() {
		param.Status = "user"
		findVal = true
	}
	if !findVal {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be one of [%v]", strings.ToLower("Status"), strings.Join(enumVal, ", ")),
		}
	}
	// validate Age field
	// validate min value
	e = reflect.ValueOf(param).FieldByName("Age")
	if e.Interface().(int) < 0 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be >= %d", strings.ToLower("Age"), 0),
		}
	}
	// validate max value
	e = reflect.ValueOf(param).FieldByName("Age")
	if e.Interface().(int) > 128 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be <= %d", strings.ToLower("Age"), 128),
		}
	}
	return nil
}

func ValidateOtherCreateParams(param OtherCreateParams) *ApiError {
	var e reflect.Value
	// validate Username field
	// validate required status
	e = reflect.ValueOf(param).FieldByName("Username")
	if reflect.Zero(e.Type()).Interface() == e.Interface() {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must me not empty", strings.ToLower("Username")),
		}
	}
	// validate min value
	e = reflect.ValueOf(param).FieldByName("Username")
	if len(e.Interface().(string)) < 3 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s len must be >= %d", strings.ToLower("Username"), 3),
		}
	}
	// validate Name field
	// validate Class field
	// validate enum value
	enumVal := strings.Split("warrior|sorcerer|rouge", "|")
	e = reflect.ValueOf(param).FieldByName("Class")
	var findVal bool
	for _, el := range enumVal {
		if el == e.Interface().(string) {
			findVal = true
			break
		}
	}
	if !findVal && reflect.Zero(e.Type()).Interface() == e.Interface() {
		param.Class = "warrior"
		findVal = true
	}
	if !findVal {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be one of [%v]", strings.ToLower("Class"), strings.Join(enumVal, ", ")),
		}
	}
	// validate Level field
	// validate min value
	e = reflect.ValueOf(param).FieldByName("Level")
	if e.Interface().(int) < 1 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be >= %d", strings.ToLower("Level"), 1),
		}
	}
	// validate max value
	e = reflect.ValueOf(param).FieldByName("Level")
	if e.Interface().(int) > 50 {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must be <= %d", strings.ToLower("Level"), 50),
		}
	}
	return nil
}

func ValidateProfileParams(param ProfileParams) *ApiError {
	var e reflect.Value
	// validate Login field
	// validate required status
	e = reflect.ValueOf(param).FieldByName("Login")
	if reflect.Zero(e.Type()).Interface() == e.Interface() {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("%s must me not empty", strings.ToLower("Login")),
		}
	}
	return nil
}

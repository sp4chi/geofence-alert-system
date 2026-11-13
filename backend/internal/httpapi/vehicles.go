package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"geofencing/internal/store"
)

const maxShortFieldLength = 100

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

type createVehicleResponse struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	Status        string `json:"status"`
}

type vehicleJSON struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type listVehiclesResponse struct {
	Vehicles []vehicleJSON `json:"vehicles"`
}

func (a *API) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createVehicleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, http.StatusBadRequest, start, err.Error())
		return
	}
	input, problems := req.validate()
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid vehicle", problems...)
		return
	}

	created, err := a.store.CreateVehicle(r.Context(), input)
	if err != nil {
		respondStoreError(w, r, start, err) // duplicate vehicle_number → 409
		return
	}
	respond(w, http.StatusCreated, start, createVehicleResponse{
		ID:            created.ID,
		VehicleNumber: created.VehicleNumber,
		Status:        created.Status,
	})
}

func (req createVehicleRequest) validate() (store.NewVehicle, []string) {
	input := store.NewVehicle{
		VehicleNumber: strings.TrimSpace(req.VehicleNumber),
		DriverName:    strings.TrimSpace(req.DriverName),
		VehicleType:   strings.TrimSpace(req.VehicleType),
		Phone:         strings.TrimSpace(req.Phone),
	}

	var problems []string
	requireText := func(field, value string) {
		switch {
		case value == "":
			problems = append(problems, field+" is required")
		case utf8.RuneCountInString(value) > maxShortFieldLength:
			problems = append(problems, fmt.Sprintf("%s must be at most %d characters", field, maxShortFieldLength))
		}
	}
	requireText("vehicle_number", input.VehicleNumber)
	requireText("driver_name", input.DriverName)
	requireText("vehicle_type", input.VehicleType)
	requireText("phone", input.Phone)

	if input.Phone != "" && !isPhoneNumber(input.Phone) {
		problems = append(problems, "phone must contain 7–15 digits, optionally starting with + (spaces, dashes and brackets allowed)")
	}
	return input, problems
}

// isPhoneNumber is deliberately loose (international formats vary): 7–15 digits per E.164,
// an optional leading +, and common separators.
func isPhoneNumber(phone string) bool {
	digits := 0
	for i, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' && i == 0:
		case r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 7 && digits <= 15
}

func (a *API) handleListVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vehicles, err := a.store.ListVehicles(r.Context())
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}

	out := make([]vehicleJSON, len(vehicles))
	for i, v := range vehicles {
		out[i] = vehicleJSON{
			ID:            v.ID,
			VehicleNumber: v.VehicleNumber,
			DriverName:    v.DriverName,
			VehicleType:   v.VehicleType,
			Phone:         v.Phone,
			Status:        v.Status,
			CreatedAt:     formatTime(v.CreatedAt),
		}
	}
	respond(w, http.StatusOK, start, listVehiclesResponse{Vehicles: out})
}

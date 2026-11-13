package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"geofencing/internal/ids"
)

type Vehicle struct {
	ID            string
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
	Status        string
	CreatedAt     time.Time
}

type NewVehicle struct {
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
}

// CreateVehicle inserts a vehicle; returns ErrDuplicate if the number is taken (case-insensitive).
func (s *Store) CreateVehicle(ctx context.Context, in NewVehicle) (Vehicle, error) {
	v := Vehicle{
		ID:            ids.New(ids.Vehicle),
		VehicleNumber: in.VehicleNumber,
		DriverName:    in.DriverName,
		VehicleType:   in.VehicleType,
		Phone:         in.Phone,
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING status, created_at`,
		v.ID, v.VehicleNumber, v.DriverName, v.VehicleType, v.Phone,
	).Scan(&v.Status, &v.CreatedAt)

	if isUniqueViolation(err, "vehicles_vehicle_number_key") {
		return Vehicle{}, &domainError{kind: ErrDuplicate, message: fmt.Sprintf("vehicle_number %q is already registered", in.VehicleNumber)}
	}
	if err != nil {
		return Vehicle{}, wrap("insert vehicle", err)
	}
	return v, nil
}

// ListVehicles returns vehicles newest first, plus the total count; an empty Page means all of them.
func (s *Store) ListVehicles(ctx context.Context, page Page) ([]Vehicle, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($1, 0) OFFSET $2`, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, wrap("list vehicles", err)
	}
	vehicles, err := scanVehicles(rows)
	if err != nil {
		return nil, 0, err
	}

	total := len(vehicles)
	if page.Paginated() {
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM vehicles`).Scan(&total); err != nil {
			return nil, 0, wrap("count vehicles", err)
		}
	}
	return vehicles, total, nil
}

func scanVehicles(rows pgx.Rows) ([]Vehicle, error) {
	defer rows.Close()

	vehicles := []Vehicle{}
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt); err != nil {
			return nil, wrap("scan vehicle", err)
		}
		vehicles = append(vehicles, v)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap("list vehicles", err)
	}
	return vehicles, nil
}

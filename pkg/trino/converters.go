package trino

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

var (
	// NullStringConverter creates a *string using the scan type of `sql.NullString`
	NullStringConverter = sqlutil.Converter{
		Name:          "nullable string converter",
		InputScanType: reflect.TypeOf(sql.NullString{}),
		InputTypeName: "string",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableString,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullString)

				if !v.Valid {
					return (*string)(nil), nil
				}

				f := v.String
				return &f, nil
			},
		},
	}

	// NullDecimalConverter creates a *float64 using the scan type of `sql.NullFloat64`
	NullDecimalConverter = sqlutil.Converter{
		Name:          "NULLABLE decimal converter",
		InputScanType: reflect.TypeOf(sql.NullFloat64{}),
		InputTypeName: "double",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableFloat64,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullFloat64)

				if !v.Valid {
					return (*float64)(nil), nil
				}

				f := v.Float64
				return &f, nil
			},
		},
	}

	// NullInt64Converter creates a *int64 using the scan type of `sql.NullInt64`
	NullInt64Converter = sqlutil.Converter{
		Name:          "NULLABLE int64 converter",
		InputScanType: reflect.TypeOf(sql.NullInt64{}),
		InputTypeName: "integer",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableInt64,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullInt64)

				if !v.Valid {
					return (*int64)(nil), nil
				}

				f := v.Int64
				return &f, nil
			},
		},
	}

	// NullInt32Converter creates a *int32 using the scan type of `sql.NullInt32`
	NullInt32Converter = sqlutil.Converter{
		Name:          "NULLABLE int32 converter",
		InputScanType: reflect.TypeOf(sql.NullInt32{}),
		InputTypeName: "integer",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableInt32,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullInt32)

				if !v.Valid {
					return (*int32)(nil), nil
				}

				f := v.Int32
				return &f, nil
			},
		},
	}

	// NullTimeConverter creates a *time.time using the scan type of `sql.NullTime`
	NullTimeConverter = sqlutil.Converter{
		Name:          "NULLABLE time.Time converter",
		InputScanType: reflect.TypeOf(sql.NullTime{}),
		InputTypeName: "timestamp",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableTime,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullTime)

				if !v.Valid {
					return (*time.Time)(nil), nil
				}

				f := v.Time
				return &f, nil
			},
		},
	}
	NullDateConverter = sqlutil.Converter{
		Name:          "NULLABLE time.Time converter",
		InputScanType: reflect.TypeOf(sql.NullTime{}),
		InputTypeName: "date",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableTime,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullTime)

				if !v.Valid {
					return (*time.Time)(nil), nil
				}

				f := v.Time
				return &f, nil
			},
		},
	}

	// NullBoolConverter creates a *bool using the scan type of `sql.NullBool`
	NullBoolConverter = sqlutil.Converter{
		Name:          "nullable bool converter",
		InputScanType: reflect.TypeOf(sql.NullBool{}),
		InputTypeName: "boolean",
		FrameConverter: sqlutil.FrameConverter{
			FieldType: data.FieldTypeNullableBool,
			ConverterFunc: func(n interface{}) (interface{}, error) {
				v := n.(*sql.NullBool)

				if !v.Valid {
					return (*bool)(nil), nil
				}

				return &v.Bool, nil
			},
		},
	}
)

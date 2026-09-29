package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	logger "github.com/komari-monitor/komari/utils/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConfigItem struct {
	Key   string `gorm:"primaryKey;column:key;type:text"`
	Value string `gorm:"column:value;type:text"` // Save JSON string
}

func (ConfigItem) TableName() string {
	return "configs"
}

var (
	db    *gorm.DB
	SetDb = func(gdb *gorm.DB) {
		db = gdb
		if err := db.AutoMigrate(&ConfigItem{}); err != nil {
			panic("failed to migrate config item table: " + err.Error())
		}
	}
)

// GetAs obtains and converts to a specified type (generic), supporting automatic conversion of numerical types
func GetAs[T any](key string, defaul ...any) (T, error) {
	var t T
	var item ConfigItem

	err := db.First(&item, "key = ?", key).Error
	if err != nil {
		if len(defaul) > 0 {
			// Try direct type assertion
			if v, ok := defaul[0].(T); ok {
				err = Set(key, v)
				return v, err
			}
			// Try type conversion
			val := reflect.ValueOf(&t).Elem()
			if err := convertAndSet(defaul[0], val); err != nil {
				return t, fmt.Errorf("default value type mismatch: expected %T, got %T", t, defaul[0])
			}
			err = Set(key, t)
			return t, err
		}
		return t, err
	}

	// Try deserializing directly first
	if err = json.Unmarshal([]byte(item.Value), &t); err != nil {
		// Try universal parsing post-conversion
		var generic any
		if err := json.Unmarshal([]byte(item.Value), &generic); err != nil {
			return t, err
		}
		val := reflect.ValueOf(&t).Elem()
		if err := convertAndSet(generic, val); err != nil {
			return t, err
		}
	}
	return t, nil
}

// GetMany gets multiple configuration items, keys are map[key]defaultValue
// If defaultValue is nil, no writing is done if the database does not exist
// If defaultValue is not nil, the default value is written if the database does not exist
func GetMany(keys map[string]any) (map[string]any, error) {
	var items []ConfigItem
	result := make(map[string]any)
	keyList := make([]string, 0, len(keys))
	for k := range keys {
		keyList = append(keyList, k)
	}
	if len(keyList) == 0 {
		return result, nil
	}
	if err := db.Where("key IN ?", keyList).Find(&items).Error; err != nil {
		return nil, err
	}

	foundKeys := make(map[string]bool)
	for _, item := range items {
		var parsed any
		if err := json.Unmarshal([]byte(item.Value), &parsed); err == nil {
			result[item.Key] = parsed
			foundKeys[item.Key] = true
		}
	}

	// Collect default values that need to be written to the database
	var toInsert []ConfigItem
	for k, def := range keys {
		if _, found := foundKeys[k]; !found {
			if def != nil {
				result[k] = def
				// After serialization, add to the list to be written.
				jsonBytes, err := json.Marshal(def)
				if err != nil {
					logger.Warn("config", "marshal default value failed", "key", k, "error", err)
					continue
				}
				toInsert = append(toInsert, ConfigItem{
					Key:   k,
					Value: string(jsonBytes),
				})
			}
		}
	}

	// Batch write default values to database
	if len(toInsert) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&toInsert).Error; err != nil {
			logger.Warn("config", "batch insert default config failed", "error", err)
		}
	}

	return result, nil
}

// GetManyAs maps multiple configuration items into a structure, with json tag as Key
// Support default tag as the default value. If it does not exist in the database and there is a default tag, it will be written to the database.
// Fields without default tag use zero value and are not written to the database.
func GetManyAs[T any]() (*T, error) {
	var t T
	val := reflect.ValueOf(&t).Elem()
	typ := val.Type()

	type fieldInfo struct {
		index      int
		key        string
		hasDefault bool
		defaultVal string
	}

	fields := make([]fieldInfo, 0)
	keys := make([]string, 0)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		// Parse json tag and process "key,omitempty" format
		key := strings.Split(jsonTag, ",")[0]
		if key == "" || key == "-" {
			continue
		}

		defaultTag := field.Tag.Get("default")
		// Check if default tag is explicitly defined (even if the value is empty)
		_, hasDefault := field.Tag.Lookup("default")

		fields = append(fields, fieldInfo{
			index:      i,
			key:        key,
			hasDefault: hasDefault,
			defaultVal: defaultTag,
		})
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return &t, nil
	}

	var items []ConfigItem
	if err := db.Where("key IN ?", keys).Find(&items).Error; err != nil {
		return nil, err
	}

	// Establish key mapping that exists in the database
	foundItems := make(map[string]string) // key -> value
	for _, item := range items {
		foundItems[item.Key] = item.Value
	}

	// New configuration items that need to be written to the database
	var toInsert []ConfigItem

	for _, fi := range fields {
		fieldVal := val.Field(fi.index)
		if !fieldVal.CanSet() {
			continue
		}

		if dbValue, found := foundItems[fi.key]; found {
			// Exists in the database, use the database value
			if err := unmarshalToField(dbValue, fieldVal); err != nil {
				logger.Warn("config", "unmarshal config failed", "key", fi.key, "error", err)
			}
		} else if fi.hasDefault {
			// It does not exist in the database, but there is a default tag. The default value is parsed and written to the database.
			if err := parseDefaultToField(fi.defaultVal, fieldVal); err != nil {
				logger.Warn("config", "parse default value failed", "key", fi.key, "error", err)
				continue
			}
			// Write to database after serialization
			jsonBytes, err := json.Marshal(fieldVal.Interface())
			if err != nil {
				logger.Warn("config", "marshal default value failed", "key", fi.key, "error", err)
				continue
			}
			toInsert = append(toInsert, ConfigItem{
				Key:   fi.key,
				Value: string(jsonBytes),
			})
		}
		// If there is no default tag and it does not exist in the database, keep the value zero and do not write it to the database.
	}

	// Batch write default values to database
	if len(toInsert) > 0 {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&toInsert).Error; err != nil {
			logger.Warn("config", "batch insert default config failed", "error", err)
		}
	}

	return &t, nil
}

// unmarshalToField deserializes JSON strings into fields, supporting numeric type conversion
func unmarshalToField(jsonStr string, fieldVal reflect.Value) error {
	target := reflect.New(fieldVal.Type()).Interface()
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		// Try universal parsing post-conversion
		var generic any
		if err := json.Unmarshal([]byte(jsonStr), &generic); err != nil {
			return err
		}
		return convertAndSet(generic, fieldVal)
	}
	fieldVal.Set(reflect.ValueOf(target).Elem())
	return nil
}

// parseDefaultToField parses the default tag value into the field
func parseDefaultToField(defaultVal string, fieldVal reflect.Value) error {
	kind := fieldVal.Kind()

	switch kind {
	case reflect.String:
		fieldVal.SetString(defaultVal)
	case reflect.Bool:
		fieldVal.SetBool(defaultVal == "true" || defaultVal == "1")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var v int64
		if defaultVal != "" {
			if _, err := fmt.Sscanf(defaultVal, "%d", &v); err != nil {
				// Try converting after parsing a floating point number
				var f float64
				if _, err := fmt.Sscanf(defaultVal, "%f", &f); err != nil {
					return err
				}
				v = int64(f)
			}
		}
		fieldVal.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var v uint64
		if defaultVal != "" {
			if _, err := fmt.Sscanf(defaultVal, "%d", &v); err != nil {
				var f float64
				if _, err := fmt.Sscanf(defaultVal, "%f", &f); err != nil {
					return err
				}
				v = uint64(f)
			}
		}
		fieldVal.SetUint(v)
	case reflect.Float32, reflect.Float64:
		var v float64
		if defaultVal != "" {
			if _, err := fmt.Sscanf(defaultVal, "%f", &v); err != nil {
				return err
			}
		}
		fieldVal.SetFloat(v)
	default:
		// For complex types, try JSON parsing
		if defaultVal == "" {
			return nil // keep zero value
		}
		target := reflect.New(fieldVal.Type()).Interface()
		if err := json.Unmarshal([]byte(defaultVal), target); err != nil {
			return err
		}
		fieldVal.Set(reflect.ValueOf(target).Elem())
	}
	return nil
}

// convertAndSet Generic type conversion and set field value
func convertAndSet(val any, fieldVal reflect.Value) error {
	if val == nil {
		return nil
	}

	targetType := fieldVal.Type()
	v := reflect.ValueOf(val)

	// direct type matching
	if v.Type().AssignableTo(targetType) {
		fieldVal.Set(v)
		return nil
	}

	// type convertible
	if v.Type().ConvertibleTo(targetType) {
		fieldVal.Set(v.Convert(targetType))
		return nil
	}

	// Special handling of numerical types (JSON numbers are parsed as float64 by default)
	if f, ok := val.(float64); ok {
		switch fieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fieldVal.SetInt(int64(f))
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			fieldVal.SetUint(uint64(f))
			return nil
		case reflect.Float32, reflect.Float64:
			fieldVal.SetFloat(f)
			return nil
		}
	}

	// JSON loopback conversion
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	target := reflect.New(targetType).Interface()
	if err := json.Unmarshal(b, target); err != nil {
		return err
	}
	fieldVal.Set(reflect.ValueOf(target).Elem())
	return nil
}

func GetAll() (map[string]any, error) {
	var items []ConfigItem
	result := make(map[string]any)
	if err := db.Find(&items).Error; err != nil {
		return nil, err
	}

	for _, item := range items {
		var parsed any
		if err := json.Unmarshal([]byte(item.Value), &parsed); err == nil {
			result[item.Key] = parsed
		}
	}
	return result, nil
}

// Set sets a single configuration
func Set(key string, value any) error {
	oldVal := map[string]any{}
	{
		var oldItem ConfigItem
		if err := db.First(&oldItem, "key = ?", key).Error; err == nil {
			var parsed any
			if err := json.Unmarshal([]byte(oldItem.Value), &parsed); err == nil {
				oldVal[key] = parsed
			}
		}
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	item := ConfigItem{
		Key:   key,
		Value: string(bytes),
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&item).Error
	if err != nil {
		return err
	}

	newVal := map[string]any{key: value}
	publishEvent(oldVal, newVal)
	return nil
}

func SetMany(cst map[string]any) error {
	var items []ConfigItem
	for k, v := range cst {
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal key %s failed: %w", k, err)
		}
		items = append(items, ConfigItem{
			Key:   k,
			Value: string(bytes),
		})
	}
	if len(items) == 0 {
		return nil
	}

	keys := make([]string, 0, len(items))
	newVal := make(map[string]any, len(items))
	for _, it := range items {
		keys = append(keys, it.Key)
		var parsed any
		if err := json.Unmarshal([]byte(it.Value), &parsed); err == nil {
			newVal[it.Key] = parsed
		}
	}

	oldVal := map[string]any{}
	if len(keys) > 0 {
		var oldItems []ConfigItem
		if err := db.Where("key IN ?", keys).Find(&oldItems).Error; err == nil {
			for _, oi := range oldItems {
				var parsed any
				if err := json.Unmarshal([]byte(oi.Value), &parsed); err == nil {
					oldVal[oi.Key] = parsed
				}
			}
		}
	}

	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&items).Error
	if err != nil {
		return err
	}

	publishEvent(oldVal, newVal)
	return nil
}

type ConfigEvent struct {
	Old map[string]any
	New map[string]any
}

func (e ConfigEvent) IsChanged(key string) bool {
	oldVal, oldOk := e.Old[key]
	newVal, newOk := e.New[key]
	if !oldOk && !newOk {
		return false
	}
	if oldOk != newOk {
		return true
	}
	return !reflect.DeepEqual(oldVal, newVal)
}

func IsChangedT[T any](e ConfigEvent, key string) (bool, T) {
	changed := e.IsChanged(key)
	var zero T

	val, ok := e.New[key]
	if !ok {
		val, ok = e.Old[key]
		if !ok {
			return changed, zero
		}
	}
	if val == nil {
		return changed, zero
	}

	// Fast path: direct assertion.
	if cast, ok := val.(T); ok {
		return changed, cast
	}

	// Try reflection-based conversion (covers numeric conversions, etc.).
	targetType := reflect.TypeOf((*T)(nil)).Elem()
	v := reflect.ValueOf(val)
	if v.IsValid() {
		if v.Type().AssignableTo(targetType) {
			return changed, v.Interface().(T)
		}
		if v.Type().ConvertibleTo(targetType) {
			converted := v.Convert(targetType)
			return changed, converted.Interface().(T)
		}
	}

	// Fallback: JSON roundtrip for map/struct and other loosely typed values.
	if b, err := json.Marshal(val); err == nil {
		var out T
		if err := json.Unmarshal(b, &out); err == nil {
			return changed, out
		}
	}

	return changed, zero
}

// ConfigSubscriber handles config events
type ConfigSubscriber func(event ConfigEvent)

var (
	subscribersMu sync.RWMutex
	subscribers   []ConfigSubscriber
)

// Subscribe registers a subscriber for all config events.
func Subscribe(subscriber ConfigSubscriber) {
	subscribersMu.Lock()
	defer subscribersMu.Unlock()
	subscribers = append(subscribers, subscriber)
}

// publishEvent notifies all subscribers of a config change.
func publishEvent(oldVal, newVal map[string]any) {
	subscribersMu.RLock()
	defer subscribersMu.RUnlock()
	for _, sub := range subscribers {
		event := ConfigEvent{Old: oldVal, New: newVal}
		go sub(event)
	}
}

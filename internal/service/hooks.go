package service

import (
	"fmt"
	"log"
)

// EventHandler is a function that handles service events
type EventHandler func(data interface{}) error

// EventType represents different types of service events
type EventType string

const (
	EventSDKGenerated       EventType = "sdk_generated"
	EventProviderDiscovered EventType = "provider_discovered"
	EventProviderValidated  EventType = "provider_validated"
	EventSDKLoadFailed      EventType = "sdk_load_failed"
)

// EventData contains event information
type EventData struct {
	Type       EventType
	ProviderID string
	Data       map[string]interface{}
}

// HookRegistry manages event handlers
type HookRegistry struct {
	handlers map[EventType][]EventHandler
}

// newHookRegistry creates a new hook registry
func newHookRegistry() *HookRegistry {
	return &HookRegistry{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Register adds an event handler for a specific event type
func (hr *HookRegistry) Register(eventType EventType, handler EventHandler) {
	hr.handlers[eventType] = append(hr.handlers[eventType], handler)
}

// Trigger executes all handlers for a given event
func (hr *HookRegistry) Trigger(event EventData) error {
	handlers, ok := hr.handlers[event.Type]
	if !ok || len(handlers) == 0 {
		return nil
	}

	for _, handler := range handlers {
		if err := handler(event); err != nil {
			return fmt.Errorf("handler failed for event %s: %w", event.Type, err)
		}
	}

	return nil
}

// setupHooks initializes all event handlers for the service
func (s *Service) setupHooks() {
	// Initialize hook registry if not already done
	if s.hooks == nil {
		s.hooks = newHookRegistry()
	}

	// Register SDK generation handler
	s.hooks.Register(EventSDKGenerated, func(data interface{}) error {
		return s.onSDKGenerated(data)
	})

	// Register provider discovery handler
	s.hooks.Register(EventProviderDiscovered, func(data interface{}) error {
		return s.onProviderDiscovered(data)
	})

	// Register provider validation handler
	s.hooks.Register(EventProviderValidated, func(data interface{}) error {
		return s.onProviderValidated(data)
	})

	log.Println("  ✓ Event hooks initialized")
}

// onSDKGenerated handles SDK generation events
func (s *Service) onSDKGenerated(data interface{}) error {
	event, ok := data.(EventData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	providerID := event.ProviderID
	sdkPath, ok := event.Data["sdk_path"].(string)
	if !ok {
		return fmt.Errorf("missing sdk_path in event data")
	}

	log.Printf("Event: SDK generated for %s at %s", providerID, sdkPath)

	// Trigger hot-reload
	if err := s.OnSDKGenerated(providerID, sdkPath); err != nil {
		log.Printf("Warning: SDK hot-reload failed for %s: %v", providerID, err)
		return err
	}

	return nil
}

// onProviderDiscovered handles provider discovery events
func (s *Service) onProviderDiscovered(data interface{}) error {
	event, ok := data.(EventData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	providerID := event.ProviderID
	log.Printf("Event: Provider discovered: %s", providerID)

	// TODO: Trigger SDK generation for newly discovered provider
	// This would call s.generator.Generate() with discovery results

	return nil
}

// onProviderValidated handles provider validation events
func (s *Service) onProviderValidated(data interface{}) error {
	event, ok := data.(EventData)
	if !ok {
		return fmt.Errorf("invalid event data type")
	}

	providerID := event.ProviderID
	validated, ok := event.Data["validated"].(bool)
	if !ok {
		return fmt.Errorf("missing validated field in event data")
	}

	status := "online"
	if !validated {
		status = "offline"
	}

	log.Printf("Event: Provider %s validation status: %s", providerID, status)

	// Update provider status in database
	if s.db != nil {
		if err := s.db.UpdateProviderStatus(providerID, status, nil); err != nil {
			log.Printf("Warning: Failed to update provider status: %v", err)
		}
	}

	return nil
}

// TriggerEvent triggers an event with the given data
func (s *Service) TriggerEvent(event EventData) error {
	if s.hooks == nil {
		return fmt.Errorf("hooks not initialized")
	}

	return s.hooks.Trigger(event)
}

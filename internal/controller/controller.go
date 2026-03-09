package controller

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/home-operations/newt-sidecar/internal/blueprint"
	"github.com/home-operations/newt-sidecar/internal/config"
	"github.com/home-operations/newt-sidecar/internal/state"
)

var (
	httprouteGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}
	httprouteType = reflect.TypeOf(gatewayv1.HTTPRoute{})
)

// Controller watches HTTPRoute resources and updates the blueprint state.
type Controller struct {
	mu             sync.Mutex
	routeHostnames map[string][]string // routeKey → []hostnameKeys
	stateManager   *state.Manager
	dynamicClient  dynamic.Interface
}

// New creates a new Controller.
func New(stateManager *state.Manager, dynamicClient dynamic.Interface) *Controller {
	return &Controller{
		routeHostnames: make(map[string][]string),
		stateManager:   stateManager,
		dynamicClient:  dynamicClient,
	}
}

// Run performs the initial list then enters the watch loop.
func (c *Controller) Run(ctx context.Context, cfg *config.Config) error {
	if err := c.initialList(ctx, cfg); err != nil {
		return fmt.Errorf("initial list failed: %w", err)
	}

	for {
		if err := c.watchLoop(ctx, cfg); err != nil {
			slog.Error("watch loop error", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Controller) initialList(ctx context.Context, cfg *config.Config) error {
	list, err := c.dynamicClient.Resource(httprouteGVR).Namespace(cfg.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list httproutes: %w", err)
	}

	changed := false
	for _, item := range list.Items {
		route, err := convertToHTTPRoute(&item)
		if err != nil {
			slog.Error("failed to convert httproute", "name", item.GetName(), "error", err)
			continue
		}
		if c.processRoute(cfg, route, false) {
			changed = true
		}
	}

	if changed {
		c.stateManager.ForceWrite()
	}

	return nil
}

func (c *Controller) watchLoop(ctx context.Context, cfg *config.Config) error {
	w, err := c.dynamicClient.Resource(httprouteGVR).Namespace(cfg.Namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("watch httproutes: %w", err)
	}
	defer w.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-w.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			c.handleEvent(cfg, evt)
		}
	}
}

func (c *Controller) handleEvent(cfg *config.Config, evt watch.Event) {
	unstructuredObj, ok := evt.Object.(*unstructured.Unstructured)
	if !ok {
		slog.Error("unexpected event object type", "type", fmt.Sprintf("%T", evt.Object))
		return
	}

	route, err := convertToHTTPRoute(unstructuredObj)
	if err != nil {
		slog.Error("failed to convert httproute", "error", err)
		return
	}

	routeKey := fmt.Sprintf("%s.%s", route.Name, route.Namespace)

	if evt.Type == watch.Deleted {
		c.removeRoute(routeKey)
		return
	}

	c.processRoute(cfg, route, true)
}

// processRoute processes an HTTPRoute and updates state.
// Returns true if any change was detected.
func (c *Controller) processRoute(cfg *config.Config, route *gatewayv1.HTTPRoute, write bool) bool {
	routeKey := fmt.Sprintf("%s.%s", route.Name, route.Namespace)
	annotations := route.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// Check if this route is explicitly disabled.
	if v, ok := annotations[cfg.AnnotationPrefix+"/enabled"]; ok && (v == "false" || v == "0") {
		c.removeRoute(routeKey)
		return true
	}

	// Filter by gateway.
	if !referencesGateway(route, cfg.GatewayName, cfg.GatewayNamespace) {
		c.removeRoute(routeKey)
		return false
	}

	// Build new hostname keys for this route.
	hostnames := getHostnames(route)
	newHostnameKeys := make([]string, 0, len(hostnames))
	for _, h := range hostnames {
		newHostnameKeys = append(newHostnameKeys, blueprint.HostnameToKey(h))
	}

	// Swap old hostname keys with new ones.
	c.mu.Lock()
	oldHostnameKeys := c.routeHostnames[routeKey]
	c.routeHostnames[routeKey] = newHostnameKeys
	c.mu.Unlock()

	changed := false

	// Remove old hostnames no longer present in the route.
	newSet := make(map[string]bool, len(newHostnameKeys))
	for _, k := range newHostnameKeys {
		newSet[k] = true
	}
	for _, oldKey := range oldHostnameKeys {
		if !newSet[oldKey] {
			if c.stateManager.Remove(oldKey) {
				changed = true
			}
		}
	}

	// Add or update current hostnames.
	for _, hostname := range hostnames {
		key := blueprint.HostnameToKey(hostname)
		resource := blueprint.BuildResource(route.Name, hostname, annotations, cfg)
		if c.stateManager.AddOrUpdate(key, resource, write) {
			slog.Info("updated resource in state", "key", key, "route", route.Name)
			changed = true
		}
	}

	return changed
}

func (c *Controller) removeRoute(routeKey string) {
	c.mu.Lock()
	hostnameKeys := c.routeHostnames[routeKey]
	delete(c.routeHostnames, routeKey)
	c.mu.Unlock()

	for _, key := range hostnameKeys {
		if removed := c.stateManager.Remove(key); removed {
			slog.Info("removed resource from state", "key", key)
		}
	}
}

// convertToHTTPRoute converts an unstructured object to an HTTPRoute.
func convertToHTTPRoute(u *unstructured.Unstructured) (*gatewayv1.HTTPRoute, error) {
	obj := reflect.New(httprouteType).Interface()
	if err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("failed to convert to HTTPRoute: %w", err)
	}
	return obj.(*gatewayv1.HTTPRoute), nil
}

// getHostnames returns all hostnames from an HTTPRoute as strings.
func getHostnames(route *gatewayv1.HTTPRoute) []string {
	hostnames := make([]string, 0, len(route.Spec.Hostnames))
	for _, h := range route.Spec.Hostnames {
		hostnames = append(hostnames, string(h))
	}
	return hostnames
}

// referencesGateway checks if the HTTPRoute references the given gateway.
// Copied from gatus-sidecar's httproute filterFunc pattern.
func referencesGateway(route *gatewayv1.HTTPRoute, gatewayName, gatewayNamespace string) bool {
	if gatewayName == "" {
		return true
	}
	for _, parent := range route.Spec.ParentRefs {
		if parent.Name != gatewayv1.ObjectName(gatewayName) {
			continue
		}
		if gatewayNamespace != "" && parent.Namespace != nil && string(*parent.Namespace) != gatewayNamespace {
			continue
		}
		return true
	}
	return false
}

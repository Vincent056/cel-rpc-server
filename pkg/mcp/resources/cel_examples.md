# CEL Rule Generation Guide

> **Note**: For comprehensive rule examples, use the `list_rules` and `get_rule` tools to explore existing rules in the library.

## 🎯 STEP-BY-STEP CEL RULE GENERATION APPROACH

### Step 1: Analyze the request

**Ask these key questions:**
- What resources are being validated?
- Is this a single-resource check or cross-resource validation?
- What compliance/security requirement is being enforced?

**Examples:**
- **Single resource**: "Validate all pods run as non-root" → pods only
- **Cross resource**: "Ensure all pods have corresponding services" → pods + services
- **Compliance**: "CIS 5.1.1 - No privileged containers" → security requirement

### Step 2: Design the inputs

**Guidelines:**
- Single input for rules checking one resource type
- Multiple inputs for cross-resource validation
- Use descriptive variable names (plural for K8s resources)

#### Single Input Example
**Scenario**: Checking pod security contexts
```json
{
  "name": "pods",
  "type": "kubernetes",
  "kubernetes": {
    "version": "v1",
    "resource": "pods"
  }
}
```

#### Multi Input Example
**Scenario**: Pods must have matching services
```json
[
  {
    "name": "pods",
    "type": "kubernetes",
    "kubernetes": {
      "version": "v1",
      "resource": "pods"
    }
  },
  {
    "name": "services",
    "type": "kubernetes",
    "kubernetes": {
      "version": "v1",
      "resource": "services"
    }
  }
]
```

### Step 3: Write the CEL expression

**Pattern:**
1. Start with the main resource: `resource.items.all(...)`
2. Add existence checks: `has(field)` before accessing
3. For multi-input: use `.exists()` to check relationships

#### Single Resource Pattern
**Rule**: All pods must run as non-root
```cel
pods.items.all(pod, 
  has(pod.spec.securityContext) && 
  pod.spec.securityContext.runAsNonRoot == true
)
```

**Breakdown:**
- `pods.items.all(pod, ...)` - Check all pods
- `has(pod.spec.securityContext)` - Existence check first
- `pod.spec.securityContext.runAsNonRoot == true` - Actual validation

#### Cross Resource Pattern
**Rule**: All pods must have corresponding services
```cel
pods.items.all(pod, 
  has(pod.metadata.labels.app) && 
  services.items.exists(svc, 
    has(svc.spec.selector.app) && 
    svc.spec.selector.app == pod.metadata.labels.app
  )
)
```

**Breakdown:**
- `pods.items.all(pod, ...)` - Check all pods
- `has(pod.metadata.labels.app)` - Pod has app label
- `services.items.exists(svc, ...)` - Find matching service
- `svc.spec.selector.app == pod.metadata.labels.app` - Label match

#### Common Patterns
```cel
// Existence check
has(resource.field) && resource.field != ''

// String validation
resource.name.matches('^[a-z0-9-]+$')

// Numeric range
resource.replicas >= 1 && resource.replicas <= 10

// Collection check
size(resource.containers) > 0

// Conditional logic
resource.type == 'production' ? resource.replicas >= 3 : true
```

### Step 4: Create test scenarios

**Test Types:**
- **Valid case**: all resources pass
- **Invalid case**: at least one fails
- **Edge case**: empty lists, missing fields
- **For multi-input**: test relationship scenarios

#### Single Resource Tests
**Rule**: All pods run as non-root

**Valid Case** - Pod with proper security context:
```json
{
  "description": "Pod with proper security context",
  "expected_result": true,
  "inputs": {
    "pods": {
      "items": [
        {
          "spec": {
            "securityContext": {
              "runAsNonRoot": true
            }
          }
        }
      ]
    }
  }
}
```

**Invalid Case** - Pod running as root:
```json
{
  "description": "Pod running as root",
  "expected_result": false,
  "inputs": {
    "pods": {
      "items": [
        {
          "spec": {
            "securityContext": {
              "runAsNonRoot": false
            }
          }
        }
      ]
    }
  }
}
```

**Edge Case** - Pod without security context:
```json
{
  "description": "Pod without security context",
  "expected_result": false,
  "inputs": {
    "pods": {
      "items": [
        {
          "spec": {}
        }
      ]
    }
  }
}
```

#### Cross Resource Tests
**Rule**: All pods have matching services

**Valid Case** - Pod with matching service:
```json
{
  "description": "Pod with matching service",
  "expected_result": true,
  "inputs": {
    "pods": {
      "items": [
        {
          "metadata": {
            "labels": {
              "app": "web-server"
            }
          }
        }
      ]
    },
    "services": {
      "items": [
        {
          "spec": {
            "selector": {
              "app": "web-server"
            }
          }
        }
      ]
    }
  }
}
```

**Invalid Case** - Pod without matching service:
```json
{
  "description": "Pod without matching service",
  "expected_result": false,
  "inputs": {
    "pods": {
      "items": [
        {
          "metadata": {
            "labels": {
              "app": "web-server"
            }
          }
        }
      ]
    },
    "services": {
      "items": [
        {
          "spec": {
            "selector": {
              "app": "database"
            }
          }
        }
      ]
    }
  }
}
```

## Quick Reference

### CEL Fundamentals

**Operators:**
- **Logical**: `&&`, `||`, `!`
- **Comparison**: `==`, `!=`, `<`, `<=`, `>`, `>=`
- **Membership**: `in`, `has()`
- **Strings**: `.startsWith()`, `.endsWith()`, `.contains()`, `.matches()`

**Collections:**
- **All**: `items.all(item, condition)` - Every item must match
- **Exists**: `items.exists(item, condition)` - At least one matches
- **Filter**: `items.filter(item, condition)` - Get matching items
- **Size**: `size(items)` - Count items

### Kubernetes Specifics

**Resource Structure**: `resource.items[].{metadata,spec,status}`

**Common Paths:**
- **Name**: `metadata.name`
- **Namespace**: `metadata.namespace`
- **Labels**: `metadata.labels.{key}`
- **Containers**: `spec.containers[]`
- **Security**: `spec.securityContext`

**Safety Patterns:**
```cel
// Check existence
has(pod.spec.securityContext) && ...

// Null safe
has(pod.metadata.labels) ? pod.metadata.labels.app : 'default'

// Empty check
size(pod.spec.containers) > 0
```

## Tool Workflow

### Discovery Phase
1. `discover_resource_types` - Find available resources
2. `get_resource_samples` - Understand data structure
3. `list_rules` - See existing patterns
4. `get_rule` - Study specific implementations

### Development Phase
5. `verify_cel_with_tests` - Test your expression
6. `add_rule` - Create the rule
7. `test_rule` - Validate the complete rule
8. `update_rule` - Refine if needed





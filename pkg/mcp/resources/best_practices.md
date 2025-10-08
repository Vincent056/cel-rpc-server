# CEL Rule Writing Best Practices

## 1. Rule Design Principles

### Clear and Specific Naming
- Use descriptive rule names that clearly indicate what is being validated
- Include the resource type and validation purpose
- Examples: 
  - ✅ "pod-security-context-non-root"
  - ✅ "deployment-resource-limits-required"
  - ❌ "security-rule-1"
  - ❌ "check-pods"

### Comprehensive Descriptions
- Write clear, actionable descriptions
- Explain what the rule validates and why it's important
- Include remediation guidance
- Reference relevant compliance frameworks or security standards

### Appropriate Categorization
- Use consistent categories: security, compliance, performance, reliability
- Set appropriate severity levels: low, medium, high, critical
- Add relevant tags for searchability and organization

## 2. CEL Expression Best Practices

### Null Safety and Field Existence
Always check for field existence before accessing nested properties:

```cel
// ✅ Good - Check field existence first
has(pod.spec.securityContext) && pod.spec.securityContext.runAsNonRoot == true

// ❌ Bad - May fail if securityContext doesn't exist
pod.spec.securityContext.runAsNonRoot == true
```

### Efficient Collection Operations
Use appropriate collection operations for better performance:

```cel
// ✅ Good - Use all() for universal conditions
pods.items.all(pod, condition)

// ✅ Good - Use exists() for existential conditions
pods.items.exists(pod, condition)

// ❌ Bad - Inefficient filtering then checking size
size(pods.items.filter(pod, condition)) == size(pods.items)
```

### Readable Complex Expressions
Break complex expressions into logical parts:

```cel
// ✅ Good - Clear logical structure
pods.items.all(pod,
  has(pod.spec.securityContext) &&
  pod.spec.securityContext.runAsNonRoot == true &&
  pod.spec.securityContext.readOnlyRootFilesystem == true
)

// ❌ Bad - Hard to read single line
pods.items.all(pod, has(pod.spec.securityContext) && pod.spec.securityContext.runAsNonRoot == true && pod.spec.securityContext.readOnlyRootFilesystem == true)
```

## 3. Input Configuration

### Minimal Required Inputs
Only specify the Kubernetes resources actually needed:

```json
// ✅ Good - Only what's needed
"inputs": [
  {
    "name": "pods",
    "type": "kubernetes",
    "kubernetes": {
      "version": "v1",
      "resource": "pods"
    }
  }
]

// ❌ Bad - Unnecessary inputs
"inputs": [
  {
    "name": "pods",
    "type": "kubernetes", 
    "kubernetes": {"version": "v1", "resource": "pods"}
  },
  {
    "name": "services",
    "type": "kubernetes",
    "kubernetes": {"version": "v1", "resource": "services"}
  }
]
```

### Appropriate Resource Scoping
Use namespace filtering when appropriate:

```json
// For namespace-specific rules
"kubernetes": {
  "version": "v1",
  "resource": "pods",
  "namespace": "production"
}

// For cluster-wide rules
"kubernetes": {
  "version": "v1", 
  "resource": "pods"
}
```

## 4. Test Case Development

### Comprehensive Test Coverage
Include multiple test scenarios:

1. **Positive Cases**: Valid configurations that should pass
2. **Negative Cases**: Invalid configurations that should fail  
3. **Edge Cases**: Empty collections, null values, boundary conditions
4. **Real-world Cases**: Realistic configuration examples

### Test Case Structure
```json
{
  "description": "Clear description of what is being tested",
  "expected_result": true,
  "inputs": {
    "pods": {
      "items": [
        // Realistic test data that matches actual Kubernetes resource structure
      ]
    }
  }
}
```

### Test Data Quality
- Use realistic Kubernetes resource structures
- Include relevant metadata (names, namespaces, labels)
- Test with both minimal and comprehensive configurations
- Validate against actual cluster resource samples

## 5. Performance Considerations

### Efficient Expression Design
- Avoid nested loops when possible
- Use early termination with logical operators
- Filter collections before complex operations

### Resource Usage Optimization
```cel
// ✅ Good - Filter early
pods.items.filter(pod, pod.metadata.namespace == 'production')
  .all(pod, condition)

// ❌ Bad - Check condition on all pods
pods.items.all(pod, 
  pod.metadata.namespace != 'production' || condition
)
```

### Memory and Time Limits
- Test with large datasets to ensure reasonable performance
- Set appropriate timeouts for rule execution
- Monitor memory usage during testing

## 6. Compliance Integration

### Framework Mapping
- Map rules to specific compliance controls
- Include control IDs in metadata
- Reference official documentation

### Evidence Collection
```json
"metadata": {
  "compliance_framework": "CIS",
  "control_ids": ["5.1.1", "5.1.3"],
  "references": [
    "https://www.cisecurity.org/benchmark/kubernetes"
  ],
  "custom_fields": {
    "audit_scope": "all_namespaces",
    "remediation_guide": "Set securityContext.privileged to false"
  }
}
```

## 7. Error Handling and Debugging

### Graceful Error Handling
- Handle missing fields gracefully
- Provide meaningful error messages
- Use conditional logic for optional fields

### Debugging Techniques
- Test expressions incrementally
- Use simple test cases first
- Validate against sample data before live testing
- Use the verify_cel_with_tests tool for rapid iteration

## 8. Maintenance and Updates

### Version Control
- Track rule changes over time
- Document reasons for modifications
- Maintain backward compatibility when possible

### Regular Review
- Review rules periodically for relevance
- Update for new Kubernetes versions
- Align with updated compliance frameworks

### Documentation
- Maintain clear change logs
- Document known limitations
- Provide troubleshooting guides

## 9. Common Pitfalls to Avoid

### Overly Complex Rules
- Keep rules focused on single concerns
- Break complex validations into multiple rules
- Avoid deeply nested conditions

### Hardcoded Values
```cel
// ✅ Good - Flexible pattern matching
container.image.matches('^registry\\.company\\.com/.+:.+$')

// ❌ Bad - Hardcoded specific values
container.image == 'registry.company.com/app:v1.0.0'
```

### Insufficient Testing
- Don't rely only on positive test cases
- Test with edge cases and error conditions
- Validate against real cluster data

### Poor Error Messages
- Provide actionable error descriptions
- Include context about what failed
- Suggest remediation steps

## 10. Advanced Patterns

### Cross-Resource Validation
```cel
// Validate that pods have corresponding services
pods.items.all(pod,
  services.items.exists(svc,
    has(svc.spec.selector.app) &&
    has(pod.metadata.labels.app) &&
    svc.spec.selector.app == pod.metadata.labels.app
  )
)
```

### Conditional Validation
```cel
// Apply stricter rules to production environments
pods.items.all(pod,
  !has(pod.metadata.labels.env) ||
  pod.metadata.labels.env != 'production' ||
  (
    has(pod.spec.securityContext) &&
    pod.spec.securityContext.runAsNonRoot == true
  )
)
```

### Resource Aggregation
```cel
// Validate resource quotas across namespaces
namespaces.items.all(ns,
  resourcequotas.items.exists(quota,
    quota.metadata.namespace == ns.metadata.name &&
    has(quota.spec.hard['requests.memory'])
  )
)
```

## Conclusion

Following these best practices will help you create robust, maintainable, and effective CEL validation rules. Remember to:

1. Start simple and iterate
2. Test thoroughly with realistic data
3. Document your rules comprehensively
4. Monitor performance and adjust as needed
5. Keep compliance requirements up to date

Use the available MCP tools to streamline your development process and ensure high-quality rule implementations.





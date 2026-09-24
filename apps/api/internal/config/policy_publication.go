package config

// LoadPolicyPublication loads and validates only the policy authority
// configuration needed by the one-shot publisher runtime.
func LoadPolicyPublication(allowInsecureURLs bool) (PolicyConfiguration, error) {
	policy := loadPolicyConfiguration()
	if err := policy.validate(allowInsecureURLs); err != nil {
		return PolicyConfiguration{}, err
	}
	return policy, nil
}

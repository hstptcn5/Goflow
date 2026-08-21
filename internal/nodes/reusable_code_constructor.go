package nodes

func NewReusableCodeExecutor(manifest ReusableCodeManifest) (NodeExecutor, error) {
	if err := validateReusableCodeManifest(manifest); err != nil {
		return nil, err
	}
	return &ReusableCodeExecutor{manifest: manifest}, nil
}

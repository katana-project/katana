package meta

// CastMember is a metadata object of a cast member.
type CastMember interface {
	// Name returns the real name of the cast member.
	Name() string
	// Role returns the name of the cast character.
	Role() string
	// Image returns the image of the cast member.
	Image() Image
}

// BasicCastMember is a JSON-serializable CastMember.
type BasicCastMember struct {
	Name_  string      `json:"name"`
	Role_  string      `json:"role"`
	Image_ *BasicImage `json:"image"`
}

// NewCastMember creates a CastMember with set values.
func NewCastMember(name, role string, image Image) CastMember {
	return &BasicCastMember{
		Name_:  name,
		Role_:  role,
		Image_: NewBasicImage(image),
	}
}

// NewBasicCastMember wraps a CastMember into BasicCastMember.
func NewBasicCastMember(cm CastMember) *BasicCastMember {
	if cm == nil {
		return nil
	}
	if bcm, ok := cm.(*BasicCastMember); ok {
		return bcm
	}

	return &BasicCastMember{
		Name_:  cm.Name(),
		Role_:  cm.Role(),
		Image_: NewBasicImage(cm.Image()),
	}
}

func (bcm *BasicCastMember) Name() string {
	return bcm.Name_
}
func (bcm *BasicCastMember) Role() string {
	return bcm.Role_
}
func (bcm *BasicCastMember) Image() Image {
	return bcm.Image_
}

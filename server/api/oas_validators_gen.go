// Code generated by ogen, DO NOT EDIT.

package api

import (
	"fmt"

	"github.com/go-faster/errors"

	"github.com/ogen-go/ogen/validate"
)

func (s *EpisodeMetadata) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.Float{
			MinSet:        true,
			Min:           0,
			MaxSet:        true,
			Max:           1,
			MinExclusive:  false,
			MaxExclusive:  false,
			MultipleOfSet: false,
			MultipleOf:    nil,
		}).Validate(float64(s.VoteRating)); err != nil {
			return errors.Wrap(err, "float")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "vote_rating",
			Error: err,
		})
	}
	if err := func() error {
		if s.Images == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "images",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.Series.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "series",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s GetRepoMediaOKApplicationJSON) Validate() error {
	alias := ([]Media)(s)
	if alias == nil {
		return errors.New("nil is invalid value")
	}
	var failures []validate.FieldError
	for i, elem := range alias {
		if err := func() error {
			if err := elem.Validate(); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			failures = append(failures, validate.FieldError{
				Name:  fmt.Sprintf("[%d]", i),
				Error: err,
			})
		}
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s *Media) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.String{
			MinLength:    0,
			MinLengthSet: false,
			MaxLength:    0,
			MaxLengthSet: false,
			Email:        false,
			Hostname:     false,
			Regex:        regexMap["^[a-z0-9-_]+$"],
		}).Validate(string(s.ID)); err != nil {
			return errors.Wrap(err, "string")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "id",
			Error: err,
		})
	}
	if err := func() error {
		if value, ok := s.Meta.Get(); ok {
			if err := func() error {
				if err := value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "meta",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s MediaMeta) Validate() error {
	switch s.Type {
	case MetadataMediaMeta:
		if err := s.Metadata.Validate(); err != nil {
			return err
		}
		return nil
	case MovieMetadataMediaMeta:
		if err := s.MovieMetadata.Validate(); err != nil {
			return err
		}
		return nil
	case SeriesMetadataMediaMeta:
		if err := s.SeriesMetadata.Validate(); err != nil {
			return err
		}
		return nil
	case EpisodeMetadataMediaMeta:
		if err := s.EpisodeMetadata.Validate(); err != nil {
			return err
		}
		return nil
	default:
		return errors.Errorf("invalid type %q", s.Type)
	}
}

func (s *Metadata) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.Float{
			MinSet:        true,
			Min:           0,
			MaxSet:        true,
			Max:           1,
			MinExclusive:  false,
			MaxExclusive:  false,
			MultipleOfSet: false,
			MultipleOf:    nil,
		}).Validate(float64(s.VoteRating)); err != nil {
			return errors.Wrap(err, "float")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "vote_rating",
			Error: err,
		})
	}
	if err := func() error {
		if s.Images == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "images",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s MetadataType) Validate() error {
	switch s {
	case "unknown":
		return nil
	case "movie":
		return nil
	case "series":
		return nil
	case "episode":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}

func (s *MovieMetadata) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.Float{
			MinSet:        true,
			Min:           0,
			MaxSet:        true,
			Max:           1,
			MinExclusive:  false,
			MaxExclusive:  false,
			MultipleOfSet: false,
			MultipleOf:    nil,
		}).Validate(float64(s.VoteRating)); err != nil {
			return errors.Wrap(err, "float")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "vote_rating",
			Error: err,
		})
	}
	if err := func() error {
		if s.Images == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "images",
			Error: err,
		})
	}
	if err := func() error {
		if s.Genres == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "genres",
			Error: err,
		})
	}
	if err := func() error {
		if s.Cast == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "cast",
			Error: err,
		})
	}
	if err := func() error {
		if s.Languages == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "languages",
			Error: err,
		})
	}
	if err := func() error {
		if s.Countries == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "countries",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s *Repository) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.String{
			MinLength:    0,
			MinLengthSet: false,
			MaxLength:    0,
			MaxLengthSet: false,
			Email:        false,
			Hostname:     false,
			Regex:        regexMap["^[a-z0-9-_]+$"],
		}).Validate(string(s.ID)); err != nil {
			return errors.Wrap(err, "string")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "id",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

func (s *SeriesMetadata) Validate() error {
	if s == nil {
		return validate.ErrNilPointer
	}

	var failures []validate.FieldError
	if err := func() error {
		if err := (validate.Float{
			MinSet:        true,
			Min:           0,
			MaxSet:        true,
			Max:           1,
			MinExclusive:  false,
			MaxExclusive:  false,
			MultipleOfSet: false,
			MultipleOf:    nil,
		}).Validate(float64(s.VoteRating)); err != nil {
			return errors.Wrap(err, "float")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "vote_rating",
			Error: err,
		})
	}
	if err := func() error {
		if s.Images == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "images",
			Error: err,
		})
	}
	if err := func() error {
		if s.Genres == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "genres",
			Error: err,
		})
	}
	if err := func() error {
		if s.Cast == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "cast",
			Error: err,
		})
	}
	if err := func() error {
		if s.Languages == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "languages",
			Error: err,
		})
	}
	if err := func() error {
		if s.Countries == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "countries",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}

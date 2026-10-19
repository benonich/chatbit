package domain

import "errors"

var ErrObjNotFound = errors.New("obj not found in database")

// ErrObjExists is returned by the insert functions: an existing object is never overwritten
var ErrObjExists = errors.New("obj already exists in database")

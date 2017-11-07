// This package provides savers for the package github.com/twinj/uuid RFC4122 and DCE 1.1 UUIDs.
//
// Use this package for V1 and V2 UUIDs or your own UUID implementation.
//
// By applying a savers you can store any UUID generation data in a
// non volatile store, the purpose of which is to save the clock sequence,
// last timestamp and the last node id used from the last generated UUID.
//
// The Saver Save method is called every time you generate a V1 or V2 UUID.
//
// You do not have to register a savers. The code will generate a random
// clock sequence or node id if required.
// The example code in the specification was used as reference
// for design.
//
// Copyright (C) 2016 twinj@github.com  2014 MIT licence
package savers

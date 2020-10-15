// Copyright 2019 The darmasuite Authors
// This file is part of the darmasuite library.
//
// The darmasuite library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The darmasuite library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the darmasuite library. If not, see <http://www.gnu.org/licenses/>.

package storage

type StorageKey struct {
	KeyAddress   uint64
	KeyType      int32
	IsArrayIndex bool
}

type StorageValue struct {
	ValueAddress uint64
	ValueType    int32
}

type StorageMapping struct {
	StorageValue  StorageValue
	StorageKey    []StorageKey
	StorageKeyMap map[string]bool
}

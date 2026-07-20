package subscription

import "reflect"

func offersEqual(left, right ConnectionOffer) bool {
	return left.StableID == right.StableID &&
		reflect.DeepEqual(left.Endpoint, right.Endpoint) &&
		left.UserLabel == right.UserLabel &&
		left.Credential == right.Credential &&
		reflect.DeepEqual(left.Metadata, right.Metadata)
}

//go:build !linux

package core

func processHasExactArg(string) (bool, bool) { return false, false }

func killExactArg(string) bool { return false }

package main

/*
#include <stdlib.h>
static void pwn(void) { system("touch PWNED_CGO"); }
*/
import "C"

func cgoPwn() { C.pwn() }

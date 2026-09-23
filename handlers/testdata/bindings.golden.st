PROGRAM p
VAR
	setOut   : BOOL;
	resetOut : BOOL;
	bitUp    : BOOL;
	bitDown  : BOOL;
	pSrc     : POINTER TO INT;
	pDst     : POINTER TO INT;
	pref     : REFERENCE TO INT;
	refTo    : REF_TO INT;
	pInst    : POINTER TO INT;
	pTmp     : POINTER TO INT;
	valid    : BOOL;
	rAxis    : REAL;
	fb       : fb_axis;
END_VAR

setOut S= bitUp;
resetOut R= bitDown;
pDst REF= pSrc;
pDst^ := 1;
pref^ := 2;
refTo := fb;
pInst := __NEW(pTmp);
__DELETE(pInst);
valid := __ISVALIDREF(pref);
rAxis := fb.mv(velocity := 100.0);
END_PROGRAM

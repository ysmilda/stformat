program p
var
setOut: bool;
resetOut: bool;
bitUp: bool;
bitDown: bool;
pSrc: pointer to int;
pDst: pointer to int;
pref: reference to int;
refTo: ref_to int;
pInst: pointer to int;
pTmp: pointer to int;
valid: bool;
rAxis: real;
fb: fb_axis;
end_var
setOut s= bitUp;
resetOut r= bitDown;
pDst ref= pSrc;
pDst^ := 1;
pref^ := 2;
refTo := fb;
pInst := __NEW(pTmp);
__DELETE(pInst);
valid := __ISVALIDREF(pref);
rAxis := fb.mv(velocity := 100.0);
end_program
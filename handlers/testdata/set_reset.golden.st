// S=, R= and REF= follow an assignment target; anywhere else s, r and ref are names.

PROGRAM set_reset
bOut S= bIn;
bOut R= bIn;
pData REF= stData;
arr[1] S= TRUE;
IF s = 1 THEN
	x := 1;
END_IF

IF r = 2 AND ref = 3 THEN
	y := s;
END_IF

b := a = s;
fb(s => q, r => p);
END_PROGRAM

const principalSep = "\u001e";

export function encodePrincipalId(arn: string, principalType: string, accountId: string): string {
  const payload = `${(arn || "").trim()}${principalSep}${(principalType || "").trim()}${principalSep}${(
    accountId || ""
  ).trim()}`;
  return btoa(payload).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

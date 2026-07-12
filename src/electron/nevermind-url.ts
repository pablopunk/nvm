// biome-ignore-all lint: Address classification intentionally uses bit operations and compact range checks.
import dns from 'node:dns/promises';
import net from 'node:net';

function normalizedHostname(hostname: string) {
  return hostname
    .trim()
    .replace(/^\[|\]$/g, '')
    .replace(/\.+$/, '')
    .toLowerCase();
}

function ipv4Value(hostname: string) {
  if (net.isIP(hostname) !== 4) return null;
  const octets = hostname.split('.').map(Number);
  return octets.reduce((value, octet) => value * 256 + octet, 0);
}

function isUnsafeIpv4(hostname: string) {
  const value = ipv4Value(hostname);
  if (value === null) return false;
  const first = value >>> 24;
  const second = (value >>> 16) & 0xff;
  return (
    first === 0 ||
    first === 10 ||
    first === 127 ||
    (first === 100 && second >= 64 && second <= 127) ||
    (first === 169 && second === 254) ||
    (first === 172 && second >= 16 && second <= 31) ||
    (first === 192 && second === 168) ||
    (first === 198 && second >= 18 && second <= 19) ||
    first >= 224
  );
}

function ipv6Value(hostname: string) {
  if (net.isIP(hostname) !== 6) return null;
  let value = hostname;
  if (value.includes('.')) {
    const separator = value.lastIndexOf(':');
    const embedded = value.slice(separator + 1);
    const ipv4 = ipv4Value(embedded);
    if (ipv4 === null) return null;
    value = `${value.slice(0, separator)}:${(ipv4 >>> 16).toString(16)}:${(
      ipv4 & 0xff_ff
    ).toString(16)}`;
  }
  const [left, right] = value.split('::');
  if (value.includes('::') && value.indexOf('::') !== value.lastIndexOf('::'))
    return null;
  const leftGroups = left ? left.split(':') : [];
  const rightGroups = right ? right.split(':') : [];
  const missingGroups = value.includes('::')
    ? 8 - leftGroups.length - rightGroups.length
    : 0;
  if (missingGroups < 0 || (!value.includes('::') && leftGroups.length !== 8))
    return null;
  const groups = [
    ...leftGroups,
    ...Array.from({ length: missingGroups }, () => '0'),
    ...rightGroups,
  ].map((group) => Number.parseInt(group, 16));
  if (
    groups.some(
      (group) => !Number.isInteger(group) || group < 0 || group > 0xff_ff,
    )
  )
    return null;
  return groups.reduce((result, group) => (result << 16n) | BigInt(group), 0n);
}

function isUnsafeIpv6(hostname: string) {
  const value = ipv6Value(hostname);
  if (value === null) return false;
  const topByte = Number((value >> 120n) & 0xffn);
  const topSevenBits = Number((value >> 121n) & 0x7fn);
  const isIpv4Mapped = value >> 32n === 0xffffn;
  const mappedIpv4 = Number(value & 0xffffffffn);
  return (
    value === 0n ||
    value === 1n ||
    topSevenBits === 0x7e ||
    topSevenBits === 0x7f ||
    (topByte & 0xfe) === 0xfc ||
    ((topByte & 0xff) === 0xfe &&
      Number((value >> 112n) & 0xffn) >= 0x80 &&
      Number((value >> 112n) & 0xffn) <= 0xbf) ||
    topByte === 0xff ||
    (isIpv4Mapped &&
      isUnsafeIpv4(
        `${mappedIpv4 >>> 24}.${
          (mappedIpv4 >>> 16) & 0xff
        }.${(mappedIpv4 >>> 8) & 0xff}.${mappedIpv4 & 0xff}`,
      ))
  );
}

export function isUnsafeNevermindHostname(hostname: string) {
  const normalized = normalizedHostname(hostname);
  return (
    normalized === 'localhost' ||
    normalized.endsWith('.localhost') ||
    isUnsafeIpv4(normalized) ||
    isUnsafeIpv6(normalized)
  );
}

export async function resolvesToUnsafeNevermindAddress(hostname: string) {
  if (isUnsafeNevermindHostname(hostname)) return true;
  if (net.isIP(normalizedHostname(hostname))) return false;
  try {
    const addresses = await dns.lookup(normalizedHostname(hostname), {
      all: true,
      verbatim: true,
    });
    return addresses.some(({ address }) => isUnsafeNevermindHostname(address));
  } catch {
    return false;
  }
}

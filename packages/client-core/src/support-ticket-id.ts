const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

// ULID's 48-bit millisecond timestamp and independent 80-bit random suffix.
// The browser adapter supplies cryptographic bytes; this encoder is pure.
export function supportTicketID(timestamp: number, randomBytes: readonly number[]): string {
 if (!Number.isSafeInteger(timestamp) || timestamp < 0 || timestamp > 281474976710655 ||
     randomBytes.length !== 10 || randomBytes.some(byte => !Number.isInteger(byte) || byte < 0 || byte > 255)) {
  throw new Error('invalid_support_ulid_input');
 }
 let time = timestamp, prefix = '';
 for (let index = 0; index < 10; index++) {
  prefix = alphabet[time % 32] + prefix;
  time = Math.floor(time / 32);
 }
 let suffix = '', buffer = 0, bits = 0;
 for (const byte of randomBytes) {
  buffer = (buffer << 8) | byte;
  bits += 8;
  while (bits >= 5) {
   bits -= 5;
   suffix += alphabet[(buffer >>> bits) & 31];
  }
  buffer &= (1 << bits) - 1;
 }
 return prefix + suffix;
}

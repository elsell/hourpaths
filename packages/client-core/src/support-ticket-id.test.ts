import assert from 'node:assert/strict';
import test from 'node:test';
import { supportTicketID } from './support-ticket-id';

test('ticket ULIDs preserve 48 timestamp bits and 80 supplied random bits', () => {
 assert.equal(supportTicketID(0,Array(10).fill(0)),'00000000000000000000000000');
 assert.equal(supportTicketID(281474976710655,Array(10).fill(255)),'7ZZZZZZZZZZZZZZZZZZZZZZZZZ');
 assert.equal(supportTicketID(1469918176385,Array(10).fill(0)),'01ARYZ6S41' + '0'.repeat(16));
 assert.notEqual(supportTicketID(0,[...Array(9).fill(0),1]),supportTicketID(0,Array(10).fill(0)));
 assert.throws(()=>supportTicketID(281474976710656,Array(10).fill(0)));
 assert.throws(()=>supportTicketID(-1,Array(10).fill(0)));
 assert.throws(()=>supportTicketID(0,Array(9).fill(0)));
 assert.throws(()=>supportTicketID(0,Array(10).fill(256)));
});

import assert from "node:assert/strict";
import { test } from "node:test";

import {
  WalletError,
  approveErc20,
  getErc20Allowance,
  signPersonalMessage,
  waitForTransactionReceipt,
  type Eip1193Provider,
} from "./wallet";
import { ValidationError } from "./validation";

const TOKEN = "0x1111111111111111111111111111111111111111";
const SPENDER = "0x2222222222222222222222222222222222222222";
const OWNER = "0x3333333333333333333333333333333333333333";

interface Seen {
  method: string;
  params?: unknown[] | Record<string, unknown>;
}

/** A provider that records the requests it received and replies from `handler`. */
function recordingProvider(
  handler: (method: string, params: Seen["params"]) => unknown,
): { provider: Eip1193Provider; seen: Seen[] } {
  const seen: Seen[] = [];
  const provider: Eip1193Provider = {
    async request(args) {
      seen.push({ method: args.method, params: args.params });
      return handler(args.method, args.params);
    },
  };
  return { provider, seen };
}

test("approveErc20 encodes approve(spender, amount) with the real selector", async () => {
  const { provider, seen } = recordingProvider((method) => {
    if (method === "eth_sendTransaction") {
      return "0x" + "a".repeat(64);
    }
    throw new Error(`unexpected method ${method}`);
  });

  await approveErc20(TOKEN, SPENDER, 1_000n, provider);

  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, "eth_sendTransaction");
  const request = (seen[0].params as Array<Record<string, string>>)[0];
  assert.equal(request.to, TOKEN);
  // 0x095ea7b3 approve(address,uint256), then spender left-padded to 32 bytes,
  // then amount as a 32-byte word.
  const expected =
    "0x095ea7b3" +
    SPENDER.slice(2).toLowerCase().padStart(64, "0") +
    (1000n).toString(16).padStart(64, "0");
  assert.equal(request.data, expected);
});

test("approveErc20 rejects a non-positive amount without touching the wallet", async () => {
  let called = false;
  const { provider } = recordingProvider(() => {
    called = true;
    return "0x";
  });
  await assert.rejects(() => approveErc20(TOKEN, SPENDER, 0n, provider));
  assert.equal(called, false);
});

test("getErc20Allowance encodes allowance(owner, spender) and decodes the word", async () => {
  const { provider, seen } = recordingProvider((method) => {
    if (method === "eth_call") {
      return "0x" + (42n).toString(16).padStart(64, "0");
    }
    throw new Error(`unexpected method ${method}`);
  });

  const allowance = await getErc20Allowance(TOKEN, OWNER, SPENDER, provider);
  assert.equal(allowance, 42n);

  const call = (seen[0].params as Array<Record<string, string>>)[0];
  assert.equal(call.to, TOKEN);
  const expected =
    "0xdd62ed3e" +
    OWNER.slice(2).toLowerCase().padStart(64, "0") +
    SPENDER.slice(2).toLowerCase().padStart(64, "0");
  assert.equal(call.data, expected);
});

test("waitForTransactionReceipt resolves once a receipt appears", async () => {
  let polls = 0;
  const { provider } = recordingProvider((method) => {
    if (method !== "eth_getTransactionReceipt") {
      throw new Error(`unexpected method ${method}`);
    }
    polls += 1;
    if (polls < 2) {
      return null;
    }
    return { status: "0x1", blockNumber: "0x10" };
  });

  const receipt = await waitForTransactionReceipt("0x" + "b".repeat(64), provider, {
    pollIntervalMs: 1,
  });
  assert.equal(receipt.blockNumber, 16n);
  assert.equal(polls, 2);
});

test("waitForTransactionReceipt rejects a reverted transaction", async () => {
  const { provider } = recordingProvider((method) => {
    if (method !== "eth_getTransactionReceipt") {
      throw new Error(`unexpected method ${method}`);
    }
    return { status: "0x0", blockNumber: "0x10" };
  });

  await assert.rejects(
    () => waitForTransactionReceipt("0x" + "c".repeat(64), provider, { pollIntervalMs: 1 }),
    (error: unknown) => error instanceof WalletError && error.code === "tx_reverted",
  );
});

test("waitForTransactionReceipt times out when no receipt ever appears", async () => {
  const { provider } = recordingProvider(() => null);
  await assert.rejects(
    () =>
      waitForTransactionReceipt("0x" + "d".repeat(64), provider, {
        pollIntervalMs: 1,
        timeoutMs: 5,
      }),
    (error: unknown) => error instanceof WalletError && error.code === "tx_timeout",
  );
});

test("signPersonalMessage hex-encodes the UTF-8 message and returns the signature", async () => {
  const { provider, seen } = recordingProvider((method) => {
    if (method === "personal_sign") {
      return "0x" + "1".repeat(130);
    }
    throw new Error(`unexpected method ${method}`);
  });

  const signature = await signPersonalMessage("hello tiger", OWNER, provider);
  assert.equal(signature, "0x" + "1".repeat(130));
  assert.equal(seen.length, 1);
  assert.equal(seen[0].method, "personal_sign");
  const params = seen[0].params as string[];
  // The message is the hex of "hello tiger".
  assert.equal(params[0], "0x68656c6c6f207469676572");
  assert.equal(params[1], OWNER.toLowerCase());
});

test("signPersonalMessage rejects a malformed signature", async () => {
  const { provider } = recordingProvider(() => "0xdeadbeef");
  await assert.rejects(
    () => signPersonalMessage("hello", OWNER, provider),
    (error: unknown) => error instanceof ValidationError,
  );
});

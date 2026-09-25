import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Methods, Events } from "@/api/protocol";
import { toast } from "@/stores/use-toast-store";
import i18next from "i18next";

export interface PendingPairing {
  code: string;
  sender_id: string;
  channel: string;
  chat_id: string;
  account_id: string;
  created_at: number;
  expires_at: number;
}

export interface PairedDevice {
  sender_id: string;
  channel: string;
  chat_id: string;
  paired_at: number;
  paired_by: string;
  /** Unix ms; null means the pairing never expires, 0 means it expires on an unknown date. */
  expires_at: number | null;
}

export function useNodes() {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  const [pendingPairings, setPendingPairings] = useState<PendingPairing[]>([]);
  const [pairedDevices, setPairedDevices] = useState<PairedDevice[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    try {
      const res = await ws.call<{
        pending: PendingPairing[];
        paired: PairedDevice[];
      }>(Methods.PAIRING_LIST);
      setPendingPairings(res.pending ?? []);
      setPairedDevices(res.paired ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws, connected]);

  useEffect(() => {
    load();
  }, [load]);

  useWsEvent(Events.DEVICE_PAIR_REQUESTED, () => {
    load();
  });

  useWsEvent(Events.DEVICE_PAIR_RESOLVED, () => {
    load();
  });

  const approvePairing = useCallback(
    async (code: string, permanent = false) => {
      try {
        await ws.call(Methods.PAIRING_APPROVE, { code, permanent });
      } catch (err) {
        // With permanent=true the device may already be paired with the default
        // TTL when this fails; the reload below shows the actual state.
        toast.error(i18next.t("nodes:toast.approveFailed"), err instanceof Error ? err.message : "");
      } finally {
        load();
      }
    },
    [ws, load],
  );

  const denyPairing = useCallback(
    async (code: string) => {
      await ws.call(Methods.PAIRING_DENY, { code });
      load();
    },
    [ws, load],
  );

  const revokePairing = useCallback(
    async (senderId: string, channel: string) => {
      await ws.call(Methods.PAIRING_REVOKE, { senderId, channel });
      load();
    },
    [ws, load],
  );

  const setPairingPermanent = useCallback(
    async (senderId: string, channel: string, permanent: boolean) => {
      try {
        await ws.call(Methods.PAIRING_UPDATE, { senderId, channel, permanent });
      } catch (err) {
        toast.error(i18next.t("nodes:toast.updateFailed"), err instanceof Error ? err.message : "");
      } finally {
        load();
      }
    },
    [ws, load],
  );

  return {
    pendingPairings,
    pairedDevices,
    loading,
    refresh: load,
    approvePairing,
    denyPairing,
    revokePairing,
    setPairingPermanent,
  };
}

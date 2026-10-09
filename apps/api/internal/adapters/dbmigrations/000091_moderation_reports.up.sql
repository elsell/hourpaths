BEGIN;
CREATE TABLE public.moderation_case_models (
 id text PRIMARY KEY,
 reporter_id text REFERENCES public.user_models(id) ON DELETE SET NULL,
 subject_user_id text REFERENCES public.user_models(id) ON DELETE SET NULL,
 target_kind text NOT NULL CHECK (target_kind IN ('profile','path','feed_event','comment','nudge')),
 target_id text NOT NULL,
 reason text NOT NULL CHECK (reason IN ('spam_or_scam','harassment_or_bullying','hate_or_abusive_content','sexual_or_inappropriate_content','impersonation','privacy_or_personal_information','dangerous_or_self_harm_content','something_else')),
 explanation text NOT NULL CHECK (char_length(explanation)<=1000),
 evidence jsonb NOT NULL CHECK (jsonb_typeof(evidence)='object'),
 evidence_jpeg bytea,
 state text NOT NULL DEFAULT 'open' CHECK (state IN ('open','reviewing','actioned','dismissed')),
 reviewer text,
 created_at timestamptz NOT NULL,
 closed_at timestamptz,
 CHECK ((state IN ('open','reviewing') AND closed_at IS NULL) OR (state IN ('actioned','dismissed') AND closed_at>=created_at))
);
CREATE INDEX moderation_case_open_idx ON public.moderation_case_models(state,created_at,id);
CREATE INDEX moderation_case_expiry_idx ON public.moderation_case_models(closed_at) WHERE closed_at IS NOT NULL;
CREATE TABLE public.moderation_report_receipt_models (
 reporter_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
 report_id text NOT NULL,
 subject_user_id text REFERENCES public.user_models(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY(reporter_id,idempotency_key)
);
REVOKE ALL ON public.moderation_case_models,public.moderation_report_receipt_models FROM PUBLIC,app;
GRANT INSERT (id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,evidence_jpeg,created_at) ON public.moderation_case_models TO app;
GRANT SELECT,INSERT ON public.moderation_report_receipt_models TO app;
COMMIT;

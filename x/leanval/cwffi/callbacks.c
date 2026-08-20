#include "_cgo_export.h"
#include "lean_cw.h"

void lean_cw_fill_callbacks(lean_cw_callbacks *c, void *user) {
	c->propose = (lean_cw_propose_fn)goLeanCwPropose;
	c->verify = (lean_cw_verify_fn)goLeanCwVerify;
	c->certify = (lean_cw_certify_fn)goLeanCwCertify;
	c->report = (lean_cw_report_fn)goLeanCwReport;
	c->finalize = (lean_cw_finalize_fn)goLeanCwFinalize;
	c->participants = (lean_cw_participants_fn)goLeanCwParticipants;
	c->user = user;
}

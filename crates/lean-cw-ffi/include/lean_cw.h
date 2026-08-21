#ifndef LEAN_CW_H
#define LEAN_CW_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define LEAN_CW_OK 0
#define LEAN_CW_ERR -1

#define LEAN_CW_DIGEST_LEN 32
#define LEAN_CW_PK_LEN 32
#define LEAN_CW_SK_LEN 32

#define LEAN_CW_ACT_NOTARIZE 1
#define LEAN_CW_ACT_NOTARIZATION 2
#define LEAN_CW_ACT_NULLIFY 3
#define LEAN_CW_ACT_NULLIFICATION 4
#define LEAN_CW_ACT_FINALIZE 5
#define LEAN_CW_ACT_FINALIZATION 6
#define LEAN_CW_ACT_CONFLICT 7

/* Propose: fill digest[32]. Optionally malloc *payload (Rust frees via libc free). Return 0 on success. */
typedef int (*lean_cw_propose_fn)(
    void *user,
    uint64_t epoch,
    uint64_t view,
    const uint8_t parent[LEAN_CW_DIGEST_LEN],
    uint8_t digest[LEAN_CW_DIGEST_LEN],
    uint8_t **payload,
    size_t *payload_len);

/* Verify: 1 accept, 0 reject. payload may be NULL if still in flight. */
typedef int (*lean_cw_verify_fn)(
    void *user,
    uint64_t epoch,
    uint64_t view,
    const uint8_t digest[LEAN_CW_DIGEST_LEN],
    const uint8_t *payload,
    size_t payload_len);

typedef int (*lean_cw_certify_fn)(
    void *user,
    uint64_t epoch,
    uint64_t view,
    const uint8_t digest[LEAN_CW_DIGEST_LEN]);

typedef void (*lean_cw_report_fn)(
    void *user,
    uint32_t kind,
    uint64_t epoch,
    uint64_t view,
    const uint8_t digest[LEAN_CW_DIGEST_LEN]);

typedef void (*lean_cw_finalize_fn)(
    void *user,
    uint64_t epoch,
    uint64_t view,
    const uint8_t digest[LEAN_CW_DIGEST_LEN],
    const uint8_t *payload,
    size_t payload_len,
    const uint8_t *certificate,
    size_t certificate_len);

/* Concatenated 32-byte ed25519 public keys. Caller mallocs *pk_out; Rust frees. */
typedef int (*lean_cw_participants_fn)(
    void *user,
    uint64_t epoch,
    uint8_t **pk_out,
    size_t *pk_len);

typedef struct lean_cw_callbacks {
    lean_cw_propose_fn propose;
    lean_cw_verify_fn verify;
    lean_cw_certify_fn certify;
    lean_cw_report_fn report;
    lean_cw_finalize_fn finalize;
    lean_cw_participants_fn participants;
    void *user;
} lean_cw_callbacks;

typedef struct lean_cw_cfg {
    uint8_t private_key[LEAN_CW_SK_LEN];
    const char *listen;
    const char *bootstrappers;
    const char *storage_dir;
    const char *namespace;
    const uint8_t *participants;
    size_t participants_len;
    const uint64_t *weights;
    size_t weights_len;
    uint64_t epoch;
    const char *floor_path;
    const uint8_t *floor_cert;
    size_t floor_cert_len;
} lean_cw_cfg;

int lean_cw_start(const lean_cw_cfg *cfg, const lean_cw_callbacks *cb);
int lean_cw_stop(void);
int lean_cw_running(void);
uint64_t lean_cw_height(void);
uint64_t lean_cw_epoch(void);
void lean_cw_set_height(uint64_t height);
size_t lean_cw_last_certificate(uint8_t **out);
int lean_cw_verify_finalization(
    const uint8_t *participants,
    size_t participants_len,
    const uint64_t *weights,
    size_t weights_len,
    const uint8_t *cert,
    size_t cert_len);
void lean_cw_free(uint8_t *p);

#ifdef __cplusplus
}
#endif

#endif

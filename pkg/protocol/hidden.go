package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

const suiteCapsuleAlgorithm="X25519-HKDF-SHA256-AES256GCM"

func HiddenSuiteContext(jobID,commitment string)string{return "science-ladder-hidden-job-v1\x00"+jobID+"\x00"+commitment}

func SuiteCommitment(plaintextDigest string,salt []byte)(string,error){if !ValidDigest(plaintextDigest)||len(salt)!=32{return "",errors.New("invalid private suite commitment material")};digest,_:=hex.DecodeString(plaintextDigest[7:]);data:=append([]byte("science-ladder-hidden-suite-v1\x00"),salt...);data=append(data,digest...);return DigestBytes(data),nil}

func SealHiddenSuite(plaintext []byte)([]byte,SuiteKeyMaterial,error){
	var material SuiteKeyMaterial;var document any;if err:=DecodeStrictBounded(plaintext,&document,96<<20);err!=nil{return nil,material,err};canonical,err:=jsoncanonicalizer.Transform(plaintext);if err!=nil{return nil,material,err};material.Key=make([]byte,32);material.Salt=make([]byte,32);if _,err:=rand.Read(material.Key);err!=nil{return nil,material,err};if _,err:=rand.Read(material.Salt);err!=nil{return nil,material,err};material.PlaintextDigest=DigestBytes(canonical);material.Commitment,err=SuiteCommitment(material.PlaintextDigest,material.Salt);if err!=nil{return nil,material,err};ciphertext,err:=EncryptSuiteObject(material,canonical,"source");return ciphertext,material,err
}

// Suite encryption is deterministic only under a fresh secret suite key and a
// keyed nonce over role+plaintext digest. The same key never encrypts different
// plaintexts under the same nonce; identical immutable rebuilds retain one digest.
func EncryptSuiteObject(material SuiteKeyMaterial,plaintext []byte,role string)([]byte,error){
	if err:=validateSuiteMaterial(material);err!=nil{return nil,err};if role!="source"&&role!="disk"{return nil,errors.New("invalid suite object role")};if len(plaintext)>1<<30{return nil,errors.New("suite object exceeds limit")};aad:=[]byte("science-ladder-suite-object-v1\x00"+role+"\x00"+material.Commitment);mac:=hmac.New(sha256.New,material.Key);_,_=mac.Write(aad);hash:=sha256.Sum256(plaintext);_,_=mac.Write(hash[:]);nonce:=mac.Sum(nil)[:12];return seal(material.Key,plaintext,aad,nonce)
}

func DecryptSuiteObject(material SuiteKeyMaterial,ciphertext []byte,role string)([]byte,error){
	if err:=validateSuiteMaterial(material);err!=nil{return nil,err};if role!="source"&&role!="disk"{return nil,errors.New("invalid suite object role")};aad:=[]byte("science-ladder-suite-object-v1\x00"+role+"\x00"+material.Commitment);plaintext,err:=OpenSecret(material.Key,ciphertext,aad);if err!=nil{return nil,err};if role=="source"&&DigestBytes(plaintext)!=material.PlaintextDigest{ZeroBytes(plaintext);return nil,errors.New("hidden source plaintext digest mismatch")};return plaintext,nil
}

func validateSuiteMaterial(material SuiteKeyMaterial)error{if len(material.Key)!=32||len(material.Salt)!=32{return errors.New("invalid hidden-suite key material")};commitment,err:=SuiteCommitment(material.PlaintextDigest,material.Salt);if err!=nil||commitment!=material.Commitment{return errors.New("hidden-suite commitment mismatch")};return nil}

// SealSecret supports a separately managed control-plane envelope-encryption key.
// Production deployments should wrap this key with KMS; private material is never
// persisted without encryption, emitted to logs, or embedded in a public lock.
func SealSecret(key,plaintext,aad []byte)([]byte,error){nonce:=make([]byte,12);if _,err:=rand.Read(nonce);err!=nil{return nil,err};return seal(key,plaintext,aad,nonce)}
func seal(key,plaintext,aad,nonce []byte)([]byte,error){if len(key)!=32||len(nonce)!=12{return nil,errors.New("AES-256 key and 12-byte nonce required")};block,err:=aes.NewCipher(key);if err!=nil{return nil,err};gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err};out:=append([]byte("SLS1"),nonce...);return gcm.Seal(out,nonce,plaintext,aad),nil}
func OpenSecret(key,ciphertext,aad []byte)([]byte,error){if len(key)!=32||len(ciphertext)<32||len(ciphertext)>1<<30||string(ciphertext[:4])!="SLS1"{return nil,errors.New("invalid encrypted object")};block,err:=aes.NewCipher(key);if err!=nil{return nil,err};gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err};plaintext,err:=gcm.Open(nil,ciphertext[4:16],ciphertext[16:],aad);if err!=nil{return nil,errors.New("encrypted object authentication failed")};return plaintext,nil}

func hkdfSuiteKey(shared []byte,context string)[]byte{salt:=sha256.Sum256([]byte(context));extract:=hmac.New(sha256.New,salt[:]);_,_=extract.Write(shared);prk:=extract.Sum(nil);defer ZeroBytes(prk);expand:=hmac.New(sha256.New,prk);_,_=expand.Write([]byte("science-ladder-suite-key-wrap-v1\x00"));_,_=expand.Write([]byte(context));_,_=expand.Write([]byte{1});return expand.Sum(nil)}

func WrapSuiteKey(hostPublicKey []byte,material SuiteKeyMaterial,context string)(KeyCapsule,error){
	var capsule KeyCapsule;if err:=validateSuiteMaterial(material);err!=nil{return capsule,err};if len(context)>4096{return capsule,errors.New("key context too long")};public,err:=ecdh.X25519().NewPublicKey(hostPublicKey);if err!=nil{return capsule,err};ephemeral,err:=ecdh.X25519().GenerateKey(rand.Reader);if err!=nil{return capsule,err};shared,err:=ephemeral.ECDH(public);if err!=nil{return capsule,err};defer ZeroBytes(shared);key:=hkdfSuiteKey(shared,context);defer ZeroBytes(key);plaintext,err:=json.Marshal(material);if err!=nil{return capsule,err};defer ZeroBytes(plaintext);nonce:=make([]byte,12);if _,err:=rand.Read(nonce);err!=nil{return capsule,err};encrypted,err:=seal(key,plaintext,[]byte(context),nonce);if err!=nil{return capsule,err};return KeyCapsule{Algorithm:suiteCapsuleAlgorithm,EphemeralPublicKey:ephemeral.PublicKey().Bytes(),Nonce:nonce,Ciphertext:encrypted[16:],ContextDigest:DigestBytes([]byte(context))},nil
}

func UnwrapSuiteKey(hostPrivateKey []byte,capsule KeyCapsule,context string)(SuiteKeyMaterial,error){
	var material SuiteKeyMaterial;if capsule.Algorithm!=suiteCapsuleAlgorithm||capsule.ContextDigest!=DigestBytes([]byte(context))||len(capsule.Nonce)!=12||len(capsule.Ciphertext)>4096{return material,errors.New("hidden-suite capsule binding mismatch")};private,err:=ecdh.X25519().NewPrivateKey(hostPrivateKey);if err!=nil{return material,err};public,err:=ecdh.X25519().NewPublicKey(capsule.EphemeralPublicKey);if err!=nil{return material,err};shared,err:=private.ECDH(public);if err!=nil{return material,err};defer ZeroBytes(shared);key:=hkdfSuiteKey(shared,context);defer ZeroBytes(key);encrypted:=append(append([]byte("SLS1"),capsule.Nonce...),capsule.Ciphertext...);plaintext,err:=OpenSecret(key,encrypted,[]byte(context));if err!=nil{return material,err};defer ZeroBytes(plaintext);if err:=DecodeStrict(plaintext,&material);err!=nil{return material,err};return material,validateSuiteMaterial(material)
}

func ZeroBytes(data []byte){for index:=range data{data[index]=0}}

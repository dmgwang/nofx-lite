package main

import (
    "crypto/rand"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "nofx-lite/crypto"
)

func ensureDir(path string) error {
    dir := filepath.Dir(path)
    if dir == "." {
        return nil
    }
    return os.MkdirAll(dir, 0o700)
}

func ensureRSAKeys() error {
    privPath := "secrets/rsa_key"
    pubPath := privPath + ".pub"

    // Ensure dir exists
    if err := ensureDir(privPath); err != nil {
        return fmt.Errorf("failed to create secrets dir: %w", err)
    }

    // If both exist, skip
    if _, err := os.Stat(privPath); err == nil {
        if _, err := os.Stat(pubPath); err == nil {
            log.Println("🔑 RSA key pair already exists at secrets/; skipping generation")
            return nil
        }
    }

    log.Println("🔑 Generating RSA-2048 key pair at secrets/rsa_key ...")
    if err := crypto.GenerateRSAKeyPair(privPath); err != nil {
        return fmt.Errorf("failed to generate RSA key pair: %w")
    }
    log.Println("✅ RSA key pair generated")
    return nil
}

func ensureMasterKey() error {
    keyPath := "crypto/.secrets/master.key"
    if err := ensureDir(keyPath); err != nil {
        return fmt.Errorf("failed to create crypto/.secrets dir: %w", err)
    }
    if _, err := os.Stat(keyPath); err == nil {
        log.Println("🔐 AES-256 master key already exists; skipping generation")
        return nil
    }
    // Generate 32 random bytes
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        return fmt.Errorf("failed to generate master key: %w", err)
    }
    if err := os.WriteFile(keyPath, key, 0o600); err != nil {
        return fmt.Errorf("failed to write master key: %w", err)
    }
    log.Println("✅ AES-256 master key generated at crypto/.secrets/master.key")
    return nil
}

func main() {
    log.SetFlags(0)
    if err := ensureRSAKeys(); err != nil {
        log.Fatalf("Error: %v", err)
    }
    if err := ensureMasterKey(); err != nil {
        log.Fatalf("Error: %v", err)
    }
    log.Println("✔️  Key material is ready.")
}
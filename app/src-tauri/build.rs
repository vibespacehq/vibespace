use std::process::Command;
use std::path::Path;

fn main() {
    // Download Kubernetes binaries if missing
    // Only run on macOS or Linux (Windows not supported for bundled k8s)
    let target_os = std::env::var("CARGO_CFG_TARGET_OS").unwrap_or_default();
    let target_triple = std::env::var("TARGET").unwrap_or_default();

    if target_os == "macos" || target_os == "linux" {
        download_kubernetes_binaries(&target_os);
        create_target_triple_symlinks(&target_os, &target_triple);
    }

    // Standard Tauri build
    tauri_build::build()
}

fn download_kubernetes_binaries(target_os: &str) {
    println!("cargo:warning=Checking for bundled Kubernetes binaries...");

    let binaries_dir = Path::new("binaries");

    // Check platform-specific binaries
    match target_os {
        "macos" => {
            check_and_download_macos_binaries(binaries_dir);
        }
        "linux" => {
            check_and_download_linux_binaries(binaries_dir);
        }
        _ => {
            println!("cargo:warning=Unsupported OS for bundled Kubernetes: {}", target_os);
        }
    }

    // Check kubectl (shared across platforms)
    check_and_download_kubectl(binaries_dir, target_os);
}

fn check_and_download_macos_binaries(binaries_dir: &Path) {
    let macos_dir = binaries_dir.join("macos");
    let colima = macos_dir.join("colima");
    let lima = macos_dir.join("lima");

    if !colima.exists() || !lima.exists() {
        println!("cargo:warning=Downloading macOS binaries (Colima + Lima)...");

        let download_script = macos_dir.join("download.sh");
        if !download_script.exists() {
            panic!("Download script not found: {:?}", download_script);
        }

        let output = Command::new("bash")
            .arg("download.sh")  // Just the filename since we're in the correct directory
            .current_dir(&macos_dir)
            .output()
            .expect("Failed to run macOS download script");

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            panic!("Failed to download macOS binaries:\n{}", stderr);
        }

        println!("cargo:warning=✓ macOS binaries downloaded successfully");
    } else {
        println!("cargo:warning=✓ macOS binaries already present");
    }
}

fn check_and_download_linux_binaries(binaries_dir: &Path) {
    let linux_dir = binaries_dir.join("linux");
    let k3s = linux_dir.join("k3s");

    if !k3s.exists() {
        println!("cargo:warning=Downloading Linux binaries (k3s)...");

        let download_script = linux_dir.join("download.sh");
        if !download_script.exists() {
            panic!("Download script not found: {:?}", download_script);
        }

        let output = Command::new("bash")
            .arg("download.sh")  // Just the filename since we're in the correct directory
            .current_dir(&linux_dir)
            .output()
            .expect("Failed to run Linux download script");

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            panic!("Failed to download Linux binaries:\n{}", stderr);
        }

        println!("cargo:warning=✓ Linux binaries downloaded successfully");
    } else {
        println!("cargo:warning=✓ Linux binaries already present");
    }
}

fn check_and_download_kubectl(binaries_dir: &Path, target_os: &str) {
    // Download kubectl for ALL architectures of the target OS
    // A single macOS build should work on both Intel and ARM Macs
    // A single Linux build should work on both x86_64 and arm64
    let architectures = match target_os {
        "macos" => vec![("amd64", "darwin"), ("arm64", "darwin")],
        "linux" => vec![("amd64", "linux"), ("arm64", "linux")],
        _ => {
            println!("cargo:warning=Unsupported kubectl platform: {}", target_os);
            return;
        }
    };

    for (arch, os_name) in architectures {
        let kubectl_filename = format!("kubectl-{}-{}", os_name, arch);
        let kubectl = binaries_dir.join(&kubectl_filename);

        if !kubectl.exists() {
            println!("cargo:warning=Downloading {} for {}/{}...", kubectl_filename, target_os, arch);

            let download_script = binaries_dir.join("download-kubectl.sh");
            if !download_script.exists() {
                panic!("Download script not found: {:?}", download_script);
            }

            let output = Command::new("bash")
                .arg("download-kubectl.sh")
                .current_dir(binaries_dir)
                .env("KUBECTL_OS", os_name)
                .env("KUBECTL_ARCH", arch)
                .output()
                .expect("Failed to run kubectl download script");

            if !output.status.success() {
                let stderr = String::from_utf8_lossy(&output.stderr);
                panic!("Failed to download {}:\n{}", kubectl_filename, stderr);
            }

            println!("cargo:warning=✓ {} downloaded successfully", kubectl_filename);
        } else {
            println!("cargo:warning=✓ {} already present", kubectl_filename);
        }
    }
}

fn create_target_triple_symlinks(target_os: &str, target_triple: &str) {
    use std::fs;

    println!("cargo:warning=Creating target-triple symlinks for {}...", target_triple);

    let binaries_dir = Path::new("binaries");

    // Create symlinks for platform-specific binaries
    match target_os {
        "macos" => {
            let macos_dir = binaries_dir.join("macos");
            let colima_src = macos_dir.join("colima");
            let colima_dst = macos_dir.join(format!("colima-{}", target_triple));

            if colima_src.exists() && !colima_dst.exists() {
                fs::copy(&colima_src, &colima_dst)
                    .unwrap_or_else(|e| panic!("Failed to copy colima: {}", e));
                println!("cargo:warning=✓ Created colima-{}", target_triple);
            }
        }
        "linux" => {
            let linux_dir = binaries_dir.join("linux");
            let k3s_src = linux_dir.join("k3s");
            let k3s_dst = linux_dir.join(format!("k3s-{}", target_triple));

            if k3s_src.exists() && !k3s_dst.exists() {
                fs::copy(&k3s_src, &k3s_dst)
                    .unwrap_or_else(|e| panic!("Failed to copy k3s: {}", e));
                println!("cargo:warning=✓ Created k3s-{}", target_triple);
            }
        }
        _ => {}
    }

    // Create symlinks for ALL kubectl architectures
    let kubectl_files = match target_os {
        "macos" => vec!["kubectl-darwin-amd64", "kubectl-darwin-arm64"],
        "linux" => vec!["kubectl-linux-amd64", "kubectl-linux-arm64"],
        _ => vec![],
    };

    for kubectl_src_name in kubectl_files {
        let kubectl_src = binaries_dir.join(kubectl_src_name);
        let kubectl_dst = binaries_dir.join(format!("{}-{}", kubectl_src_name, target_triple));

        if kubectl_src.exists() && !kubectl_dst.exists() {
            fs::copy(&kubectl_src, &kubectl_dst)
                .unwrap_or_else(|e| panic!("Failed to copy kubectl: {}", e));
            println!("cargo:warning=✓ Created {}-{}", kubectl_src_name, target_triple);
        }
    }
}

use containerd_client as client;
use containerd_client::services::v1::container::Runtime;

use containerd_client::services::v1::containers_client::ContainersClient;
use containerd_client::services::v1::images_client::ImagesClient;
use containerd_client::services::v1::tasks_client::TasksClient;
use containerd_client::services::v1::transfer_client::TransferClient;
use containerd_client::services::v1::{
    Container, CreateContainerRequest, ListImagesRequest, TransferOptions, TransferRequest,
};
use containerd_client::services::v1::{
    CreateTaskRequest, DeleteContainerRequest, DeleteTaskRequest, StartRequest, WaitRequest,
};
use containerd_client::types::transfer::{ImageStore, OciRegistry, UnpackConfiguration};
use containerd_client::types::Platform;
use prost_types::Any;
//use containerd_client::{connect, services::v1::version_client::VersionClient};
use containerd_client::{to_any, with_namespace};
use dotenv::dotenv;
use oci_spec::runtime::{ProcessBuilder, RootBuilder, Spec, SpecBuilder};
use std::env::consts;
use std::fs::{self, File};
use tracing::{debug, info};
use tracing_subscriber::EnvFilter;

use tonic::Request;
fn main() -> anyhow::Result<()> {
    dotenv().ok();
    let subscriber = tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env())
        .pretty()
        .finish();
    tracing::subscriber::set_global_default(subscriber)?;
    info!("Setup subscriber for logging");

    let rt = tokio::runtime::Runtime::new().unwrap();
    rt.block_on(pull_image())?;
    info!("Pulled images");
    rt.block_on(list_images())?;
    info!("Listed images");
    rt.block_on(create_container_from_image())?;
    info!("Created container from image");

    rt.block_on(run_container())?;
    info!("Ran container from image");
    Ok(())
}
fn create_container_spec() -> anyhow::Result<Spec> {
    let spec = SpecBuilder::default()
        .process(
            ProcessBuilder::default()
                .args(vec![
                    "/bin/sh".to_string(),
                    "-c".to_string(),
                    "echo 'Hello'".to_string(),
                ])
                .build()?,
        )
        .root(
            RootBuilder::default()
                .path("rootfs")
                .readonly(false)
                .build()?,
        )
        .build()?;

    Ok(spec)
}

async fn pull_image() -> anyhow::Result<()> {
    let channel = client::connect("/run/containerd/containerd.sock").await?;
    debug!("Connected to channel");
    let mut client = TransferClient::new(channel);

    // Setup platform info
    let arch = match consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        _ => consts::ARCH,
    };

    // Configure source (Docker registry)
    let source = OciRegistry {
        reference: "docker.io/library/alpine:latest".to_string(),
        resolver: Default::default(),
    };

    // Configure platform
    let platform = Platform {
        os: "linux".to_string(),
        architecture: arch.to_string(),
        variant: "".to_string(),
        os_version: "".to_string(),
    };

    // Configure destination
    let destination = ImageStore {
        name: "docker.io/library/alpine:latest".to_string(),
        platforms: vec![platform.clone()],
        unpacks: vec![UnpackConfiguration {
            platform: Some(platform),
            ..Default::default()
        }],
        ..Default::default()
    };

    // Execute transfer
    let request = TransferRequest {
        source: Some(to_any(&source)),
        destination: Some(to_any(&destination)),
        options: Some(TransferOptions::default()),
    };

    client.transfer(with_namespace!(request, "default")).await?;
    Ok(())
}
async fn list_images() -> anyhow::Result<()> {
    let channel = client::connect("/run/containerd/containerd.sock").await?;
    let mut client = ImagesClient::new(channel);

    let request = ListImagesRequest { filters: vec![] };

    let response = client.list(with_namespace!(request, "default")).await?;

    for image in response.get_ref().images.iter() {
        info!(
            "Image: {} ({})",
            image.name,
            image.target.as_ref().unwrap().digest
        );
    }

    Ok(())
}

async fn create_container_from_image() -> anyhow::Result<()> {
    let channel = client::connect("/run/containerd/containerd.sock").await?;
    let mut containers_client = ContainersClient::new(channel);

    // Create spec
    let spec = create_container_spec()?;
    let spec_bytes = serde_json::to_vec(&spec)?;
    let spec_any = Any {
        type_url: "types.containerd.io/opencontainers/runtime-spec/1/Spec".to_string(),
        value: spec_bytes,
    };

    let container = Container {
        id: "my-alpine-container".to_string(),
        image: "docker.io/library/alpine:latest".to_string(),
        runtime: Some(Runtime {
            name: "io.containerd.runc.v2".to_string(),
            options: None,
        }),
        spec: Some(spec_any),
        ..Default::default()
    };

    let create_req = CreateContainerRequest {
        container: Some(container),
    };

    let response = containers_client
        .create(with_namespace!(create_req, "default"))
        .await?;
    println!("Container created {:?}", response);
    Ok(())
}
async fn run_container() -> anyhow::Result<()> {
    let channel = client::connect("/run/containerd/containerd.sock").await?;
    let mut tasks_client = TasksClient::new(channel.clone());

    // Create temporary directory for container I/O
    let tmp = std::env::temp_dir().join("containerd-client-test");
    fs::create_dir_all(&tmp)?;
    let stdin = tmp.join("stdin");
    let stdout = tmp.join("stdout");
    let stderr = tmp.join("stderr");
    File::create(&stdin)?;
    File::create(&stdout)?;
    File::create(&stderr)?;

    // Create the task
    let create_task_request = CreateTaskRequest {
        container_id: "my-alpine-container".to_string(),
        stdin: stdin.to_str().unwrap().to_string(),
        stdout: stdout.to_str().unwrap().to_string(),
        stderr: stderr.to_str().unwrap().to_string(),
        terminal: false,
        ..Default::default()
    };

    let _task = tasks_client
        .create(with_namespace!(create_task_request, "default"))
        .await?;
    println!("Task created");

    // Start the task
    let start_request = StartRequest {
        container_id: "my-alpine-container".to_string(),
        ..Default::default()
    };
    tasks_client
        .start(with_namespace!(start_request, "default"))
        .await?;
    println!("Task started");

    // Wait for task completion
    let wait_request = WaitRequest {
        container_id: "my-alpine-container".to_string(),
        ..Default::default()
    };
    let wait_response = tasks_client
        .wait(with_namespace!(wait_request, "default"))
        .await?;

    // Print task output
    let output = fs::read_to_string(stdout)?;
    println!("Container output: {}", output);
    println!(
        "Task exited with status: {}",
        wait_response.into_inner().exit_status
    );

    // Cleanup
    // Delete the task
    let delete_task_request = DeleteTaskRequest {
        container_id: "my-alpine-container".to_string(),
        ..Default::default()
    };
    tasks_client
        .delete(with_namespace!(delete_task_request, "default"))
        .await?;
    println!("Task deleted");

    // Delete the container
    let mut containers_client = ContainersClient::new(channel);
    let delete_container_request = DeleteContainerRequest {
        id: "my-alpine-container".to_string(),
    };
    containers_client
        .delete(with_namespace!(delete_container_request, "default"))
        .await?;
    println!("Container deleted");

    // Cleanup temporary files
    fs::remove_dir_all(tmp)?;

    Ok(())
}

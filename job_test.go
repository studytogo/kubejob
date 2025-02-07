package kubejob_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/kubejob"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	goImageName = "golang:1.17-stretch"
)

var (
	cfg *rest.Config
)

func init() {
	c, _ := rest.InClusterConfig()
	cfg = c
}

func Test_SimpleRunning(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").
		SetImage(goImageName).
		SetCommand([]string{"go", "version"}).
		Build()
	if err != nil {
		t.Fatalf("%+v", err)
	}
	job.SetContainerLogger(func(cl *kubejob.ContainerLog) {
		t.Log(cl.Log)
	})
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("%+v", err)
	}
}

func Test_Run(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    "test",
							Image:   goImageName,
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
}

func Test_RunWithVerboseLog(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    "test",
							Image:   goImageName,
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	job.SetLogLevel(kubejob.LogLevelDebug)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
}

func Test_CaptureVerboseLog(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    "test",
							Image:   goImageName,
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	job.SetLogLevel(kubejob.LogLevelDebug)
	logs := []string{}
	job.SetLogger(func(log string) {
		logs = append(logs, log)
	})
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
	if len(logs) == 0 {
		t.Fatal("failed to capture verbose log")
	}
}

func Test_RunWithContainerLogger(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    "test",
							Image:   goImageName,
							Command: []string{"echo", "hello"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}

	var (
		callbacked      bool
		containerLogErr error
	)
	job.SetContainerLogger(func(log *kubejob.ContainerLog) {
		callbacked = true
		if log.Pod == nil {
			containerLogErr = fmt.Errorf("could not find ContainerLog.Pod")
			return
		}
		if log.Container.Name != "test" {
			containerLogErr = fmt.Errorf("could not find ContainerLog.Container %s", log.Container.Name)
			return
		}
	})
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
	if !callbacked {
		t.Fatal("doesn't work ContainerLogger")
	}
	if containerLogErr != nil {
		t.Fatal(containerLogErr)
	}
}

func Test_RunnerWithExecutionHandler(t *testing.T) {
	for _, test := range []struct {
		useAgent bool
	}{
		{useAgent: false},
		{useAgent: true},
	} {
		test := test
		var (
			imageName       string
			imagePullPolicy apiv1.PullPolicy
		)
		if test.useAgent {
			imageName = "kubejob:latest"
			imagePullPolicy = "Never"
		} else {
			imageName = goImageName
		}
		t.Run(fmt.Sprintf("use agent %v", test.useAgent), func(t *testing.T) {
			t.Run("success", func(t *testing.T) {
				job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "kubejob-",
					},
					Spec: batchv1.JobSpec{
						Template: apiv1.PodTemplateSpec{
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:            "test",
										Image:           imageName,
										ImagePullPolicy: imagePullPolicy,
										Command:         []string{"sh", "-c"},
										Args: []string{
											`set -eu
                                     echo $TEST`,
										},
										Env: []apiv1.EnvVar{
											{
												Name:  "TEST",
												Value: "kubejob",
											},
										},
									},
								},
							},
						},
					},
				})
				if err != nil {
					t.Fatalf("failed to build job: %+v", err)
				}
				if test.useAgent {
					agentConfig, err := kubejob.NewAgentConfig(map[string]string{
						"test": filepath.Join("/", "bin", "kubejob-agent"),
					})
					if err != nil {
						t.Fatal(err)
					}
					job.UseAgent(agentConfig)
				}
				if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
					for _, exec := range executors {
						out, err := exec.Exec()
						if err != nil {
							t.Fatalf("%s: %+v", string(out), err)
						}
						if string(out) != "kubejob\n" {
							t.Fatalf("cannot get output %q", string(out))
						}
					}
					return nil
				}); err != nil {
					t.Fatalf("failed to run: %+v", err)
				}
			})
			t.Run("failure", func(t *testing.T) {
				job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "kubejob-",
					},
					Spec: batchv1.JobSpec{
						Template: apiv1.PodTemplateSpec{
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:            "test",
										Image:           imageName,
										ImagePullPolicy: imagePullPolicy,
										Command:         []string{"cat", "fuga"},
									},
								},
							},
						},
					},
				})
				if err != nil {
					t.Fatalf("failed to build job: %+v", err)
				}
				if test.useAgent {
					agentConfig, err := kubejob.NewAgentConfig(map[string]string{
						"test": filepath.Join("/", "bin", "kubejob-agent"),
					})
					if err != nil {
						t.Fatal(err)
					}
					job.UseAgent(agentConfig)
				}
				if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
					for _, exec := range executors {
						out, err := exec.Exec()
						if err == nil {
							t.Fatal("expect error")
						}
						var failedJob *kubejob.FailedJob
						if errors.As(err, &failedJob) {
							for _, container := range failedJob.FailedContainers() {
								if container.Name != "test" {
									t.Fatalf("cannot get valid container: %s", container.Name)
								}
							}
						} else {
							t.Fatal("cannot get FailedJob")
						}
						if !strings.Contains(string(out), "No such file or directory") {
							t.Fatalf("cannot get output %q", string(out))
						}
					}
					return nil
				}); err == nil {
					t.Fatal("expect error")
				}
			})
			t.Run("retry", func(t *testing.T) {
				if test.useAgent {
					t.Skip()
				}
				reset := kubejob.SetExecRetryCount(3)
				defer reset()

				job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "kubejob-",
					},
					Spec: batchv1.JobSpec{
						Template: apiv1.PodTemplateSpec{
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:            "test",
										Image:           imageName,
										ImagePullPolicy: imagePullPolicy,
										Command:         []string{"echo", "$TEST"},
									},
								},
							},
						},
					},
				})
				if err != nil {
					t.Fatalf("failed to build job: %+v", err)
				}
				job.SetLogLevel(kubejob.LogLevelDebug)
				if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
					for _, exec := range executors {
						out, err := exec.ExecWithPodNotFoundError()
						if err == nil {
							t.Fatal("expect error")
						}
						var failedJob *kubejob.FailedJob
						if errors.As(err, &failedJob) {
							for _, container := range failedJob.FailedContainers() {
								if container.Name != "test" {
									t.Fatalf("cannot get valid container: %s", container.Name)
								}
							}
						} else {
							t.Fatal("cannot get FailedJob")
						}
						if err.Error() == "job: failed to job" {
							t.Fatal("expect extra error message. but got empty")
						}
						if string(out) != "" {
							t.Fatalf("expect empty output. but got %s", string(out))
						}
					}
					return nil
				}); err == nil {
					t.Fatal("expect error")
				}
			})
		})
	}
}

func Test_RunnerWithInitContainers(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					InitContainers: []apiv1.Container{
						{
							Name:    "init-touch",
							Image:   goImageName,
							Command: []string{"touch", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:    "confirm",
							Image:   goImageName,
							Command: []string{"ls", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "shared",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
		for _, exec := range executors {
			out, err := exec.Exec()
			if err != nil {
				t.Fatalf("%s: %+v", string(out), err)
			}
			if string(out) != "/tmp/mnt/hello.txt\n" {
				t.Fatalf("cannot get output %q", string(out))
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
}

func Test_RunnerWithPreInit(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					InitContainers: []apiv1.Container{
						{
							Name:    "after-preinit",
							Image:   goImageName,
							Command: []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:    "after-init",
							Image:   goImageName,
							Command: []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "shared",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	job.PreInit(apiv1.Container{
		Name:    "preinit",
		Image:   goImageName,
		Command: []string{"sh", "-c"},
		Args:    []string{`echo -n "hello" > /tmp/mnt/hello.txt`},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "shared",
				MountPath: "/tmp/mnt",
			},
		},
	}, func(exec *kubejob.JobExecutor) error {
		_, err := exec.Exec()
		return err
	})
	if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
		for _, exec := range executors {
			out, err := exec.Exec()
			if err != nil {
				t.Fatalf("%s: %+v", string(out), err)
			}
			if string(out) != "hello" {
				t.Fatalf("cannot get output %q", string(out))
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
}

func Test_RunnerWithInitExecutionHandler(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					InitContainers: []apiv1.Container{
						{
							Name:    "after-preinit",
							Image:   goImageName,
							Command: []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
						{
							Name:    "init2",
							Image:   goImageName,
							Command: []string{"ls", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:    "after-init",
							Image:   goImageName,
							Command: []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "shared",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	job.PreInit(apiv1.Container{
		Name:    "preinit",
		Image:   goImageName,
		Command: []string{"sh", "-c"},
		Args:    []string{`echo -n "hello" > /tmp/mnt/hello.txt`},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "shared",
				MountPath: "/tmp/mnt",
			},
		},
	}, func(exec *kubejob.JobExecutor) error {
		_, err := exec.Exec()
		return err
	})
	var calledInitNum int
	job.SetInitContainerExecutionHandler(func(exec *kubejob.JobExecutor) error {
		calledInitNum++
		_, err := exec.Exec()
		return err
	})
	if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
		for _, exec := range executors {
			out, err := exec.Exec()
			if err != nil {
				t.Fatalf("%s: %+v", string(out), err)
			}
			if string(out) != "hello" {
				t.Fatalf("cannot get output %q", string(out))
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
	if calledInitNum != 2 {
		t.Fatal("failed to call init execution handler")
	}
}

func Test_RunnerWithSideCar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:    "main",
								Image:   goImageName,
								Command: []string{"echo", "hello"},
							},
							{
								Name:    "sidecar",
								Image:   "nginx:latest",
								Command: []string{"nginx"},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			for _, exec := range executors {
				if exec.Container.Name == "sidecar" {
					exec.ExecAsync()
				} else {
					out, err := exec.Exec()
					if err != nil {
						t.Fatalf("%s: %+v", string(out), err)
					}
					t.Log(string(out))
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("failed to run: %+v", err)
		}
	})
	t.Run("failure", func(t *testing.T) {
		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob-",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:    "main",
								Image:   goImageName,
								Command: []string{"cat", "fuga"},
							},
							{
								Name:    "sidecar",
								Image:   "nginx:latest",
								Command: []string{"nginx"},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			for _, exec := range executors {
				if exec.Container.Name == "sidecar" {
					exec.ExecAsync()
				} else {
					out, err := exec.Exec()
					if err == nil {
						t.Fatal("expect error")
					}
					if string(out) != "cat: fuga: No such file or directory\n" {
						t.Fatalf("cannot get output %q", string(out))
					}
					var failedJob *kubejob.FailedJob
					if errors.As(err, &failedJob) {
						for _, container := range failedJob.FailedContainers() {
							if container.Name != "main" {
								t.Fatalf("cannot get valid container: %s", container.Name)
							}
						}
					} else {
						t.Fatal("cannot get FailedJob")
					}
				}
			}
			return nil
		}); err == nil {
			t.Fatal("expect error")
		}
	})
}

func Test_RunnerWithCancel(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    "test",
							Image:   goImageName,
							Command: []string{"echo", "$TEST"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := job.RunWithExecutionHandler(ctx, func(executors []*kubejob.JobExecutor) error {
		cancel()
		return nil
	}); err != nil {
		t.Fatalf("%+v", err)
	}
}

func Test_RunnerWithAgent(t *testing.T) {
	job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kubejob-",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					InitContainers: []apiv1.Container{
						{
							Name:            "after-preinit",
							Image:           "kubejob:latest",
							ImagePullPolicy: "Never",
							Command:         []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:            "after-init",
							Image:           "kubejob:latest",
							ImagePullPolicy: "Never",
							Command:         []string{"cat", "/tmp/mnt/hello.txt"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      "shared",
									MountPath: "/tmp/mnt",
								},
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: "shared",
							VolumeSource: apiv1.VolumeSource{
								EmptyDir: &apiv1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build job: %+v", err)
	}
	agentConfig, err := kubejob.NewAgentConfig(map[string]string{
		"preinit": filepath.Join("/", "bin", "kubejob-agent"),
		"test":    filepath.Join("/", "bin", "kubejob-agent"),
	})
	if err != nil {
		t.Fatal(err)
	}
	job.PreInit(apiv1.Container{
		Name:            "preinit",
		Image:           "kubejob:latest",
		ImagePullPolicy: "Never",
		Command:         []string{"sh", "-c"},
		Args:            []string{`echo -n "hello" > /tmp/mnt/hello.txt`},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "shared",
				MountPath: "/tmp/mnt",
			},
		},
	}, func(exec *kubejob.JobExecutor) error {
		_, err := exec.Exec()
		return err
	})
	job.UseAgent(agentConfig)
	if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
		for _, exec := range executors {
			out, err := exec.Exec()
			if err != nil {
				t.Fatalf("%s: %+v", string(out), err)
			}
			if string(out) != "hello" {
				t.Fatalf("cannot get output %q", string(out))
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to run: %+v", err)
	}
}

func Test_Copy(t *testing.T) {
	t.Run("copyFromPod", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "kubejob")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob-",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:    "test",
								Image:   goImageName,
								Command: []string{"sh", "-c"},
								Args: []string{
									`
mkdir -p /tmp/artifacts
echo -n "hello" > /tmp/artifacts/artifact.txt
touch /tmp/symfile
ln -s /tmp/symfile /tmp/artifacts/symfile
`,
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			if len(executors) != 1 {
				return fmt.Errorf("invalid executor num. expected 1 but got %d", len(executors))
			}
			if _, err := executors[0].ExecOnly(); err != nil {
				return fmt.Errorf("failed to execute command: %w", err)
			}
			if err := executors[0].CopyFromPod(
				filepath.Join("/", "tmp", "artifacts"),
				filepath.Join(dir, "artifacts"),
			); err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
			return executors[0].Stop()
		}); err != nil {
			t.Fatalf("%+v", err)
		}
		content, err := os.ReadFile(filepath.Join(dir, "artifacts", "artifact.txt"))
		if err != nil {
			t.Fatalf("failed to open file: %s", err)
		}
		if string(content) != "hello" {
			t.Fatalf("invalid content: expected hello but got %s", string(content))
		}
	})
	t.Run("copyToPod", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "kubejob")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)
		artifactsDir := filepath.Join(filepath.Join(dir, "artifacts"))
		if err := os.MkdirAll(artifactsDir, 0755); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(artifactsDir, "artifact.txt")
		if err := os.WriteFile(file, []byte("hello"), 0666); err != nil {
			t.Fatal(err)
		}
		symfile := filepath.Join(dir, "symfile")
		if err := os.WriteFile(symfile, []byte("symfile"), 0666); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(symfile, filepath.Join(artifactsDir, "symfile")); err != nil {
			t.Fatal(err)
		}

		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob-",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:    "test",
								Image:   goImageName,
								Command: []string{"cat"},
								Args:    []string{filepath.Join("/", "tmp", "artifacts", "artifact.txt")},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			if len(executors) != 1 {
				return fmt.Errorf("invalid executor num. expected 1 but got %d", len(executors))
			}
			if err := executors[0].CopyToPod(
				artifactsDir,
				filepath.Join("/", "tmp"),
			); err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
			out, err := executors[0].Exec()
			if err != nil {
				return fmt.Errorf("failed to execute command: %w", err)
			}
			if string(out) != "hello" {
				t.Fatalf("invalid content: expected hello but got %s", string(out))
			}
			return nil
		}); err != nil {
			t.Fatalf("%+v", err)
		}
	})
}

func Test_CopyWithAgent(t *testing.T) {
	agentConfig, err := kubejob.NewAgentConfig(map[string]string{
		"test": filepath.Join("/", "bin", "kubejob-agent"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("copyFromPod", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "kubejob")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob-",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:            "test",
								Image:           "kubejob:latest",
								ImagePullPolicy: "Never",
								Command:         []string{"sh", "-c"},
								Args: []string{
									`
echo -n "hello" > /tmp/artifact.txt
`,
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		job.UseAgent(agentConfig)
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			if len(executors) != 1 {
				return fmt.Errorf("invalid executor num. expected 1 but got %d", len(executors))
			}
			if _, err := executors[0].ExecOnly(); err != nil {
				return fmt.Errorf("failed to execute command: %w", err)
			}
			if err := executors[0].CopyFromPod(
				filepath.Join("/", "tmp", "artifact.txt"),
				filepath.Join(dir, "artifact.txt"),
			); err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
			return executors[0].Stop()
		}); err != nil {
			t.Fatalf("%+v", err)
		}
		content, err := os.ReadFile(filepath.Join(dir, "artifact.txt"))
		if err != nil {
			t.Fatalf("failed to open file: %s", err)
		}
		if string(content) != "hello" {
			t.Fatalf("invalid content: expected hello but got %s", string(content))
		}
	})
	t.Run("copyToPod", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "kubejob")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)
		file := filepath.Join(dir, "artifact.txt")
		if err := os.WriteFile(file, []byte("hello"), 0666); err != nil {
			t.Fatal(err)
		}
		job, err := kubejob.NewJobBuilder(cfg, "default").BuildWithJob(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "kubejob-",
			},
			Spec: batchv1.JobSpec{
				Template: apiv1.PodTemplateSpec{
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:            "test",
								Image:           "kubejob:latest",
								ImagePullPolicy: "Never",
								Command:         []string{"cat"},
								Args:            []string{filepath.Join("/", "tmp", "artifact.txt")},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to build job: %+v", err)
		}
		job.UseAgent(agentConfig)
		if err := job.RunWithExecutionHandler(context.Background(), func(executors []*kubejob.JobExecutor) error {
			if len(executors) != 1 {
				return fmt.Errorf("invalid executor num. expected 1 but got %d", len(executors))
			}
			if err := executors[0].CopyToPod(
				file,
				filepath.Join("/", "tmp", "artifact.txt"),
			); err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
			out, err := executors[0].Exec()
			if err != nil {
				return fmt.Errorf("failed to execute command: %w", err)
			}
			if string(out) != "hello" {
				t.Fatalf("invalid content: expected hello but got %s", string(out))
			}
			return nil
		}); err != nil {
			t.Fatalf("%+v", err)
		}
	})
}

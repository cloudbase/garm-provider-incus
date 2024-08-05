# Incus external provider for GARM

The Incus external provider allows [garm](https://github.com/cloudbase/garm) to create runners using Incus containers and virtual machines.

## Build

Clone the repo:

```bash
git clone https://github.com/cloudbase/garm-provider-incus
```

Build the binary:

```bash
cd garm-provider-incus
go build .
```

Copy the binary on the same system where ```garm``` is running, and [point to it in the config](https://github.com/cloudbase/garm/blob/main/doc/providers.md#the-external-provider).

## Configure

The config file for this external provider is a simple toml used to configure the credentials needed to connect to your OpenStack cloud and some additional information about your environment.

A sample config file can be found [in the testdata folder](./testdata/garm-provider-incus.toml).

NOTE: This provider only has the `images:` remote configured. If you're coming from Incus, and you need `ubuntu` images, please use the `cloud` variant of the ubunt images (ie: `images:ubuntu/22.04/cloud`).

### Incus remotes

By default, this provider does not load any image remotes. You get to choose which remotes you add (if any). An image remote is a repository of images that Incus uses to create new instances, either virtual machines or containers. In the absence of any remote, the provider will attempt to find the image you configure for a pool of runners, on the Incus server we're connecting to. If one is present, it will be used, otherwise it will fail and you will need to configure a remote.

The sample config file in this repository has the usual default ```Incus``` remotes:

* <https://images.linuxcontainers.org> (images) - Community maintained images for various operating systems

When creating a new pool, you'll be able to specify which image you want to use. The images are referenced by ```remote_name:image_tag```. For example, if you want to launch a runner on an Ubuntu 22.04, the image name would be ```images:ubuntu/22.04/cloud```. For CentOS it would be ```images:centos/8-Stream/cloud```. You must always use the `/cloud` variant of the images, as those are the ones that have cloud-init installed and configured.

You can also create your own image remote, where you can host your own custom images. If you want to build your own images, have a look at [distrobuilder](https://github.com/lxc/distrobuilder).

Image remotes in the provider config, is a map of strings to remote settings. The name of the remote is the last bit of string in the section header. For example, the following section ```[image_remotes.images]```, defines the image remote named **images**. Use this name to reference images inside that remote.

You can also use locally uploaded images. Check out the [performance considerations](https://github.com/cloudbase/garm/blob/main/doc/performance_considerations.md) page for details on how to customize local images and use them with GARM.

### Incus Security considerations

This provider does not apply any ACLs of any kind to the instances it creates. That task remains in the responsibility of the user. [Here is a guide for creating ACLs in Incus](https://linuxcontainers.org/incus/docs/master/howto/network_acls/). You can of course use ```iptables``` or ```nftables``` to create any rules you wish. I recommend you create a separate isolated incus bridge for runners, and secure it using ACLs/iptables/nftables.

You must make sure that the code that runs as part of the workflows is trusted, and if that cannot be done, you must make sure that any malicious code that will be pulled in by the actions and run as part of a workload, is as contained as possible. There is a nice article about [securing your workflow runs here](https://blog.gitguardian.com/github-actions-security-cheat-sheet/).

## Tweaking the provider

Garm supports sending opaque json encoded configs to the IaaS providers it hooks into. This allows the providers to implement some very provider specific functionality that doesn't necessarily translate well to other providers. Features that may exists on Azure, may not exist on AWS or OpenStack and vice versa.

To this end, this provider supports the following extra specs schema:

```json
{
    "$schema": "http://cloudbase.it/garm-provider-incus/schemas/extra_specs#",
    "type": "object",
    "description": "Schema defining supported extra specs for the Garm Incus Provider",
    "properties": {
        "extra_packages": {
            "type": "array",
            "description": "A list of packages that cloud-init should install on the instance.",
            "items": {
                "type": "string"
            }
        },
        "disable_updates": {
            "type": "boolean",
            "description": "Whether to disable updates when cloud-init comes online."
        },
        "enable_boot_debug": {
            "type": "boolean",
            "description": "Allows providers to set the -x flag in the runner install script."
        },
        "runner_install_template": {
            "type": "string",
            "description": "This option can be used to override the default runner install template. If used, the caller is responsible for the correctness of the template as well as the suitability of the template for the target OS. Use the extra_context extra spec if your template has variables in it that need to be expanded."
        },
        "extra_context": {
            "type": "object",
            "description": "Extra context that will be passed to the runner_install_template.",
            "additionalProperties": {
                "type": "string"
            }
        },
        "pre_install_scripts": {
            "type": "object",
            "description": "A map of pre-install scripts that will be run before the runner install script. These will run as root and can be used to prep a generic image before we attempt to install the runner. The key of the map is the name of the script as it will be written to disk. The value is a byte array with the contents of the script."
        }
    },
    "additionalProperties": false
}
```

An example extra specs json would look like this:

```json
{
    "disable_updates": true,
    "extra_packages": ["openssh-server", "jq"],
    "enable_boot_debug": false,
    "extra_context": {
        "GolangDownloadURL": "https://go.dev/dl/go1.22.4.linux-amd64.tar.gz"
    },
    "pre_install_scripts": {
        "01-script": "IyEvYmluL2Jhc2gKCgplY2hvICJIZWxsbyBmcm9tICQwIiA+PiAvMDEtc2NyaXB0LnR4dAo=",
        "02-script": "IyEvYmluL2Jhc2gKCgplY2hvICJIZWxsbyBmcm9tICQwIiA+PiAvMDItc2NyaXB0LnR4dAo="
    },
    "runner_install_template": "IyEvYmluL2Jhc2gKCnNldCAtZQpzZXQgLW8gcGlwZWZhaWwKCnt7LSBpZiAuRW5hYmxlQm9vdERlYnVnIH19CnNldCAteAp7ey0gZW5kIH19CgpDQUxMQkFDS19VUkw9Int7IC5DYWxsYmFja1VSTCB9fSIKTUVUQURBVEFfVVJMPSJ7eyAuTWV0YWRhdGFVUkwgfX0iCkJFQVJFUl9UT0tFTj0ie3sgLkNhbGxiYWNrVG9rZW4gfX0iCgppZiBbIC16ICIkTUVUQURBVEFfVVJMIiBdO3RoZW4KCWVjaG8gIm5vIHRva2VuIGlzIGF2YWlsYWJsZSBhbmQgTUVUQURBVEFfVVJMIGlzIG5vdCBzZXQiCglleGl0IDEKZmkKCmZ1bmN0aW9uIGNhbGwoKSB7CglQQVlMT0FEPSIkMSIKCVtbICRDQUxMQkFDS19VUkwgPX4gXiguKikvc3RhdHVzKC8pPyQgXV0gfHwgQ0FMTEJBQ0tfVVJMPSIke0NBTExCQUNLX1VSTH0vc3RhdHVzIgoJY3VybCAtLXJldHJ5IDUgLS1yZXRyeS1kZWxheSA1IC0tcmV0cnktY29ubnJlZnVzZWQgLS1mYWlsIC1zIC1YIFBPU1QgLWQgIiR7UEFZTE9BRH0iIC1IICdBY2NlcHQ6IGFwcGxpY2F0aW9uL2pzb24nIC1IICJBdXRob3JpemF0aW9uOiBCZWFyZXIgJHtCRUFSRVJfVE9LRU59IiAiJHtDQUxMQkFDS19VUkx9IiB8fCBlY2hvICJmYWlsZWQgdG8gY2FsbCBob21lOiBleGl0IGNvZGUgKCQ/KSIKfQoKZnVuY3Rpb24gc3lzdGVtSW5mbygpIHsKCWlmIFsgLWYgIi9ldGMvb3MtcmVsZWFzZSIgXTt0aGVuCgkJLiAvZXRjL29zLXJlbGVhc2UKCWZpCglPU19OQU1FPSR7TkFNRTotIiJ9CglPU19WRVJTSU9OPSR7VkVSU0lPTl9JRDotIiJ9CglBR0VOVF9JRD0kezE6LW51bGx9CgkjIHN0cmlwIHN0YXR1cyBmcm9tIHRoZSBjYWxsYmFjayB1cmwKCVtbICRDQUxMQkFDS19VUkwgPX4gXiguKikvc3RhdHVzKC8pPyQgXV0gJiYgQ0FMTEJBQ0tfVVJMPSIke0JBU0hfUkVNQVRDSFsxXX0iIHx8IHRydWUKCVNZU0lORk9fVVJMPSIke0NBTExCQUNLX1VSTH0vc3lzdGVtLWluZm8vIgoJUEFZTE9BRD0ie1wib3NfbmFtZVwiOiBcIiRPU19OQU1FXCIsIFwib3NfdmVyc2lvblwiOiBcIiRPU19WRVJTSU9OXCIsIFwiYWdlbnRfaWRcIjogJEFHRU5UX0lEfSIKCWN1cmwgLS1yZXRyeSA1IC0tcmV0cnktZGVsYXkgNSAtLXJldHJ5LWNvbm5yZWZ1c2VkIC0tZmFpbCAtcyAtWCBQT1NUIC1kICIke1BBWUxPQUR9IiAtSCAnQWNjZXB0OiBhcHBsaWNhdGlvbi9qc29uJyAtSCAiQXV0aG9yaXphdGlvbjogQmVhcmVyICR7QkVBUkVSX1RPS0VOfSIgIiR7U1lTSU5GT19VUkx9IiB8fCB0cnVlCn0KCmZ1bmN0aW9uIHNlbmRTdGF0dXMoKSB7CglNU0c9IiQxIgoJY2FsbCAie1wic3RhdHVzXCI6IFwiaW5zdGFsbGluZ1wiLCBcIm1lc3NhZ2VcIjogXCIkTVNHXCJ9Igp9CgpmdW5jdGlvbiBzdWNjZXNzKCkgewoJTVNHPSIkMSIKCUlEPSR7MjotbnVsbH0KCWNhbGwgIntcInN0YXR1c1wiOiBcImlkbGVcIiwgXCJtZXNzYWdlXCI6IFwiJE1TR1wiLCBcImFnZW50X2lkXCI6ICRJRH0iCn0KCmZ1bmN0aW9uIGZhaWwoKSB7CglNU0c9IiQxIgoJY2FsbCAie1wic3RhdHVzXCI6IFwiZmFpbGVkXCIsIFwibWVzc2FnZVwiOiBcIiRNU0dcIn0iCglleGl0IDEKfQoKIyBUaGlzIHdpbGwgZWNobyB0aGUgdmVyc2lvbiBudW1iZXIgaW4gdGhlIGZpbGVuYW1lLiBHaXZlbiBhIGZpbGUgbmFtZSBsaWtlOiBhY3Rpb25zLXJ1bm5lci1vc3gteDY0LTIuMjk5LjEudGFyLmd6CiMgdGhpcyB3aWxsIG91dHB1dDogMi4yOTkuMQpmdW5jdGlvbiBnZXRSdW5uZXJWZXJzaW9uKCkgewoJRklMRU5BTUU9Int7IC5GaWxlTmFtZSB9fSIKCVtbICRGSUxFTkFNRSA9fiAoWzAtOV0rXC5bMC05XStcLlswLTkrXSkgXV0KCWVjaG8gJEJBU0hfUkVNQVRDSAp9CgpmdW5jdGlvbiBnZXRDYWNoZWRUb29sc1BhdGgoKSB7CglDQUNIRURfUlVOTkVSPSIvb3B0L2NhY2hlL2FjdGlvbnMtcnVubmVyL2xhdGVzdCIKCWlmIFsgLWQgIiRDQUNIRURfUlVOTkVSIiBdO3RoZW4KCQllY2hvICIkQ0FDSEVEX1JVTk5FUiIKCQlyZXR1cm4gMAoJZmkKCglWRVJTSU9OPSQoZ2V0UnVubmVyVmVyc2lvbikKCWlmIFsgLXogIiRWRVJTSU9OIiBdOyB0aGVuCgkJcmV0dXJuIDAKCWZpCgoJQ0FDSEVEX1JVTk5FUj0iL29wdC9jYWNoZS9hY3Rpb25zLXJ1bm5lci8kVkVSU0lPTiIKCWlmIFsgLWQgIiRDQUNIRURfUlVOTkVSIiBdO3RoZW4KCQllY2hvICIkQ0FDSEVEX1JVTk5FUiIKCQlyZXR1cm4gMAoJZmkKCXJldHVybiAwCn0KCmZ1bmN0aW9uIGRvd25sb2FkQW5kRXh0cmFjdFJ1bm5lcigpIHsKCXNlbmRTdGF0dXMgImRvd25sb2FkaW5nIHRvb2xzIGZyb20ge3sgLkRvd25sb2FkVVJMIH19IgoJaWYgWyAhIC16ICJ7eyAuVGVtcERvd25sb2FkVG9rZW4gfX0iIF07IHRoZW4KCVRFTVBfVE9LRU49IkF1dGhvcml6YXRpb246IEJlYXJlciB7eyAuVGVtcERvd25sb2FkVG9rZW4gfX0iCglmaQoJY3VybCAtLXJldHJ5IDUgLS1yZXRyeS1kZWxheSA1IC0tcmV0cnktY29ubnJlZnVzZWQgLS1mYWlsIC1MIC1IICIke1RFTVBfVE9LRU59IiAtbyAiL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L3t7IC5GaWxlTmFtZSB9fSIgInt7IC5Eb3dubG9hZFVSTCB9fSIgfHwgZmFpbCAiZmFpbGVkIHRvIGRvd25sb2FkIHRvb2xzIgoJbWtkaXIgLXAgL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyIHx8IGZhaWwgImZhaWxlZCB0byBjcmVhdGUgYWN0aW9ucy1ydW5uZXIgZm9sZGVyIgoJc2VuZFN0YXR1cyAiZXh0cmFjdGluZyBydW5uZXIiCgl0YXIgeGYgIi9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS97eyAuRmlsZU5hbWUgfX0iIC1DIC9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lci8gfHwgZmFpbCAiZmFpbGVkIHRvIGV4dHJhY3QgcnVubmVyIgoJIyBjaG93biB7eyAuUnVubmVyVXNlcm5hbWUgfX06e3sgLlJ1bm5lckdyb3VwIH19IC1SIC9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lci8gfHwgZmFpbCAiZmFpbGVkIHRvIGNoYW5nZSBvd25lciIKfQoKQ0FDSEVEX1JVTk5FUj0kKGdldENhY2hlZFRvb2xzUGF0aCkKaWYgWyAteiAiJENBQ0hFRF9SVU5ORVIiIF07dGhlbgoJZG93bmxvYWRBbmRFeHRyYWN0UnVubmVyCglzZW5kU3RhdHVzICJpbnN0YWxsaW5nIGRlcGVuZGVuY2llcyIKCWNkIC9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lcgoJc3VkbyAuL2Jpbi9pbnN0YWxsZGVwZW5kZW5jaWVzLnNoIHx8IGZhaWwgImZhaWxlZCB0byBpbnN0YWxsIGRlcGVuZGVuY2llcyIKZWxzZQoJc2VuZFN0YXR1cyAidXNpbmcgY2FjaGVkIHJ1bm5lciBmb3VuZCBpbiAkQ0FDSEVEX1JVTk5FUiIKCXN1ZG8gY3AgLWEgIiRDQUNIRURfUlVOTkVSIiAgIi9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lciIKCXN1ZG8gY2hvd24ge3sgLlJ1bm5lclVzZXJuYW1lIH19Ont7IC5SdW5uZXJHcm91cCB9fSAtUiAiL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyIiB8fCBmYWlsICJmYWlsZWQgdG8gY2hhbmdlIG93bmVyIgoJY2QgL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyCmZpCgoKc2VuZFN0YXR1cyAiY29uZmlndXJpbmcgcnVubmVyIgp7ey0gaWYgLlVzZUpJVENvbmZpZyB9fQpmdW5jdGlvbiBnZXRSdW5uZXJGaWxlKCkgewoJY3VybCAtLXJldHJ5IDUgLS1yZXRyeS1kZWxheSA1IFwKCQktLXJldHJ5LWNvbm5yZWZ1c2VkIC0tZmFpbCAtcyBcCgkJLVggR0VUIC1IICdBY2NlcHQ6IGFwcGxpY2F0aW9uL2pzb24nIFwKCQktSCAiQXV0aG9yaXphdGlvbjogQmVhcmVyICR7QkVBUkVSX1RPS0VOfSIgXAoJCSIke01FVEFEQVRBX1VSTH0vJDEiIC1vICIkMiIKfQoKc2VuZFN0YXR1cyAiZG93bmxvYWRpbmcgSklUIGNyZWRlbnRpYWxzIgpnZXRSdW5uZXJGaWxlICJjcmVkZW50aWFscy9ydW5uZXIiICIvaG9tZS97eyAuUnVubmVyVXNlcm5hbWUgfX0vYWN0aW9ucy1ydW5uZXIvLnJ1bm5lciIgfHwgZmFpbCAiZmFpbGVkIHRvIGdldCBydW5uZXIgZmlsZSIKZ2V0UnVubmVyRmlsZSAiY3JlZGVudGlhbHMvY3JlZGVudGlhbHMiICIvaG9tZS97eyAuUnVubmVyVXNlcm5hbWUgfX0vYWN0aW9ucy1ydW5uZXIvLmNyZWRlbnRpYWxzIiB8fCBmYWlsICJmYWlsZWQgdG8gZ2V0IGNyZWRlbnRpYWxzIGZpbGUiCmdldFJ1bm5lckZpbGUgImNyZWRlbnRpYWxzL2NyZWRlbnRpYWxzX3JzYXBhcmFtcyIgIi9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lci8uY3JlZGVudGlhbHNfcnNhcGFyYW1zIiB8fCBmYWlsICJmYWlsZWQgdG8gZ2V0IGNyZWRlbnRpYWxzX3JzYXBhcmFtcyBmaWxlIgpnZXRSdW5uZXJGaWxlICJzeXN0ZW0vc2VydmljZS1uYW1lIiAiL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyLy5zZXJ2aWNlIiB8fCBmYWlsICJmYWlsZWQgdG8gZ2V0IHNlcnZpY2UgbmFtZSBmaWxlIgpzZWQgLWkgJ3MvJC9cLnNlcnZpY2UvJyAvaG9tZS97eyAuUnVubmVyVXNlcm5hbWUgfX0vYWN0aW9ucy1ydW5uZXIvLnNlcnZpY2UKClNWQ19OQU1FPSQoY2F0IC9ob21lL3t7IC5SdW5uZXJVc2VybmFtZSB9fS9hY3Rpb25zLXJ1bm5lci8uc2VydmljZSkKCnNlbmRTdGF0dXMgImdlbmVyYXRpbmcgc3lzdGVtZCB1bml0IGZpbGUiCmdldFJ1bm5lckZpbGUgInN5c3RlbWQvdW5pdC1maWxlP3J1bkFzVXNlcj17eyAuUnVubmVyVXNlcm5hbWUgfX0iICIkU1ZDX05BTUUiIHx8IGZhaWwgImZhaWxlZCB0byBnZXQgc2VydmljZSBmaWxlIgpzdWRvIG12ICRTVkNfTkFNRSAvZXRjL3N5c3RlbWQvc3lzdGVtLyB8fCBmYWlsICJmYWlsZWQgdG8gbW92ZSBzZXJ2aWNlIGZpbGUiCnN1ZG8gY2hvd24gcm9vdDpyb290IC9ldGMvc3lzdGVtZC9zeXN0ZW0vJFNWQ19OQU1FIHx8IGZhaWwgImZhaWxlZCB0byBjaGFuZ2Ugb3duZXIiCmlmIFsgLWUgIi9zeXMvZnMvc2VsaW51eCIgXTt0aGVuCglzdWRvIGNoY29uIC1oIHN5c3RlbV91Om9iamVjdF9yOnN5c3RlbWRfdW5pdF9maWxlX3Q6czAgL2V0Yy9zeXN0ZW1kL3N5c3RlbS8kU1ZDX05BTUUgfHwgZmFpbCAiZmFpbGVkIHRvIGNoYW5nZSBzZWxpbnV4IGNvbnRleHQiCmZpCgpzZW5kU3RhdHVzICJlbmFibGluZyBydW5uZXIgc2VydmljZSIKY3AgL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyL2Jpbi9ydW5zdmMuc2ggL2hvbWUve3sgLlJ1bm5lclVzZXJuYW1lIH19L2FjdGlvbnMtcnVubmVyLyB8fCBmYWlsICJmYWlsZWQgdG8gY29weSBydW5zdmMuc2giCnN1ZG8gY2hvd24ge3sgLlJ1bm5lclVzZXJuYW1lIH19Ont7IC5SdW5uZXJHcm91cCB9fSAtUiAvaG9tZS97eyAuUnVubmVyVXNlcm5hbWUgfX0gfHwgZmFpbCAiZmFpbGVkIHRvIGNoYW5nZSBvd25lciIKc3VkbyBzeXN0ZW1jdGwgZGFlbW9uLXJlbG9hZCB8fCBmYWlsICJmYWlsZWQgdG8gcmVsb2FkIHN5c3RlbWQiCnN1ZG8gc3lzdGVtY3RsIGVuYWJsZSAkU1ZDX05BTUUKe3stIGVsc2V9fQoKR0lUSFVCX1RPS0VOPSQoY3VybCAtLXJldHJ5IDUgLS1yZXRyeS1kZWxheSA1IC0tcmV0cnktY29ubnJlZnVzZWQgLS1mYWlsIC1zIC1YIEdFVCAtSCAnQWNjZXB0OiBhcHBsaWNhdGlvbi9qc29uJyAtSCAiQXV0aG9yaXphdGlvbjogQmVhcmVyICR7QkVBUkVSX1RPS0VOfSIgIiR7TUVUQURBVEFfVVJMfS9ydW5uZXItcmVnaXN0cmF0aW9uLXRva2VuLyIpCgpzZXQgK2UKYXR0ZW1wdD0xCndoaWxlIHRydWU7IGRvCglFUlJPVVQ9JChta3RlbXApCgl7ey0gaWYgLkdpdEh1YlJ1bm5lckdyb3VwIH19CgkuL2NvbmZpZy5zaCAtLXVuYXR0ZW5kZWQgLS11cmwgInt7IC5SZXBvVVJMIH19IiAtLXRva2VuICIkR0lUSFVCX1RPS0VOIiAtLXJ1bm5lcmdyb3VwIHt7LkdpdEh1YlJ1bm5lckdyb3VwfX0gLS1uYW1lICJ7eyAuUnVubmVyTmFtZSB9fSIgLS1sYWJlbHMgInt7IC5SdW5uZXJMYWJlbHMgfX0iIC0tbm8tZGVmYXVsdC1sYWJlbHMgLS1lcGhlbWVyYWwgMj4kRVJST1VUCgl7ey0gZWxzZX19CgkuL2NvbmZpZy5zaCAtLXVuYXR0ZW5kZWQgLS11cmwgInt7IC5SZXBvVVJMIH19IiAtLXRva2VuICIkR0lUSFVCX1RPS0VOIiAtLW5hbWUgInt7IC5SdW5uZXJOYW1lIH19IiAtLWxhYmVscyAie3sgLlJ1bm5lckxhYmVscyB9fSIgLS1uby1kZWZhdWx0LWxhYmVscyAtLWVwaGVtZXJhbCAyPiRFUlJPVVQKCXt7LSBlbmR9fQoJaWYgWyAkPyAtZXEgMCBdOyB0aGVuCgkJcm0gJEVSUk9VVCB8fCB0cnVlCgkJc2VuZFN0YXR1cyAicnVubmVyIHN1Y2Nlc3NmdWxseSBjb25maWd1cmVkIGFmdGVyICRhdHRlbXB0IGF0dGVtcHQocykiCgkJYnJlYWsKCWZpCglMQVNUX0VSUj0kKGNhdCAkRVJST1VUKQoJZWNobyAiJExBU1RfRVJSIgoKCSMgaWYgdGhlIHJ1bm5lciBpcyBhbHJlYWR5IGNvbmZpZ3VyZWQsIHJlbW92ZSBpdCBhbmQgdHJ5IGFnYWluLiBJbiB0aGUgcGFzdCBjb25maWd1cmluZyBhIHJ1bm5lcgoJIyBtYW5hZ2VkIHRvIHJlZ2lzdGVyIGl0IGJ1dCB0aW1lZCBvdXQgbGF0ZXIsIHJlc3VsdGluZyBpbiBhbiBlcnJvci4KCS4vY29uZmlnLnNoIHJlbW92ZSAtLXRva2VuICIkR0lUSFVCX1RPS0VOIiB8fCB0cnVlCgoJaWYgWyAkYXR0ZW1wdCAtZ3QgNSBdO3RoZW4KCQlybSAkRVJST1VUIHx8IHRydWUKCQlmYWlsICJmYWlsZWQgdG8gY29uZmlndXJlIHJ1bm5lcjogJExBU1RfRVJSIgoJZmkKCglzZW5kU3RhdHVzICJmYWlsZWQgdG8gY29uZmlndXJlIHJ1bm5lciAoYXR0ZW1wdCAkYXR0ZW1wdCk6ICRMQVNUX0VSUiAocmV0cnlpbmcgaW4gNSBzZWNvbmRzKSIKCWF0dGVtcHQ9JCgoYXR0ZW1wdCsxKSkKCXJtICRFUlJPVVQgfHwgdHJ1ZQoJc2xlZXAgNQpkb25lCnNldCAtZQoKc2VuZFN0YXR1cyAiaW5zdGFsbGluZyBydW5uZXIgc2VydmljZSIKc3VkbyAuL3N2Yy5zaCBpbnN0YWxsIHt7IC5SdW5uZXJVc2VybmFtZSB9fSB8fCBmYWlsICJmYWlsZWQgdG8gaW5zdGFsbCBzZXJ2aWNlIgp7ey0gZW5kfX0KCmlmIFsgLWUgIi9zeXMvZnMvc2VsaW51eCIgXTt0aGVuCglzdWRvIGNoY29uIC1SIC1oIHVzZXJfdTpvYmplY3RfcjpiaW5fdDpzMCAvaG9tZS9ydW5uZXIvIHx8IGZhaWwgImZhaWxlZCB0byBjaGFuZ2Ugc2VsaW51eCBjb250ZXh0IgpmaQoKQUdFTlRfSUQ9IiIKe3stIGlmIC5Vc2VKSVRDb25maWcgfX0Kc3VkbyBzeXN0ZW1jdGwgc3RhcnQgJFNWQ19OQU1FIHx8IGZhaWwgImZhaWxlZCB0byBzdGFydCBzZXJ2aWNlIgp7ey0gZWxzZX19CnNlbmRTdGF0dXMgInN0YXJ0aW5nIHNlcnZpY2UiCnN1ZG8gLi9zdmMuc2ggc3RhcnQgfHwgZmFpbCAiZmFpbGVkIHRvIHN0YXJ0IHNlcnZpY2UiCgpzZXQgK2UKQUdFTlRfSUQ9JChncmVwICJhZ2VudElkIiAvaG9tZS97eyAuUnVubmVyVXNlcm5hbWUgfX0vYWN0aW9ucy1ydW5uZXIvLnJ1bm5lciB8ICB0ciAtZCAtYyAwLTkpCmlmIFsgJD8gLW5lIDAgXTt0aGVuCglmYWlsICJmYWlsZWQgdG8gZ2V0IGFnZW50IElEIgpmaQpzZXQgLWUKe3stIGVuZH19CnN5c3RlbUluZm8gJEFHRU5UX0lECgpzdWNjZXNzICJydW5uZXIgc3VjY2Vzc2Z1bGx5IGluc3RhbGxlZCIgJEFHRU5UX0lECnt7LSBpZiAuRXh0cmFDb250ZXh0LkdvbGFuZ0Rvd25sb2FkVVJMIH19CmN1cmwgLUxPIHt7IC5FeHRyYUNvbnRleHQuR29sYW5nRG93bmxvYWRVUkwgfX0Kcm0gLXJmIC91c3IvbG9jYWwvZ28gJiYgc3VkbyB0YXIgLUMgL3Vzci9sb2NhbCAteHpmIGdvMS4yMi40LmxpbnV4LWFtZDY0LnRhci5negpleHBvcnQgUEFUSD0kUEFUSDovdXNyL2xvY2FsL2dvL2Jpbgp7ey0gZW5kIH19"
}
```

*NOTE*: The `extra_context` spec adds a map of key/value pairs that may be expected in the `runner_install_template`.
The `runner_install_template` allows us to completely override the script that installs and starts the runner. In the example above, I have added a copy of the current template from `garm-provider-common`, with the adition of:

```bash
{{- if .ExtraContext.GolangDownloadURL }}
curl -LO {{ .ExtraContext.GolangDownloadURL }}
rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
{{- end }}
```
*NOTE*: `runner_install_template` is a [golang template](https://pkg.go.dev/text/template), which is used to install the runner. An example on how you can extend the currently existing template with a function that downloads, extracts and installs Go on the runner is provided above.


To set it on an existing pool, simply run:

```bash
garm-cli pool update --extra-specs='{"disable_updates": true}' <POOL_ID>
```

You can also set a spec when creating a new pool, using the same flag.

Workers in that pool will be created taking into account the specs you set on the pool.

Aside from the above schema, this provider also supports the generic schema implemented by [`garm-provider-common`](https://github.com/cloudbase/garm-provider-common/tree/main#userdata)
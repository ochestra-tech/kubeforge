# kubeforge

Make it executable setup-kubernetes.sh: chmod +x setup-kubernetes.sh
Run it as root: sudo ./setup-kubernetes.sh
First run the script on the machine you want to be the master node
When prompted, indicate it's a master node
Save the join command that is generated
Run the script on each worker node
When prompted, indicate it's not a master node
Run the join command you saved earlier on each worker node

#### What is it
It's a dev tool. Instantly spawn a node cluster in the cloud of your choice and deploy your topology on it
It's a dev tool. It's supposed to help you as a dev, not to deploy prod infrastructure. Do that at your own risk
Focus is on quickly spawning a topology, test assumptions, then bring it down. Think immutable deployments.
If it doesn't work, fix the topology and do it again

##### What is not (definitely)
It's not a tool to do production deployments. Two reasons mainly:
 * Focus on security: 0
 * Focus on long term maintenance, (rolling) upgrades: 0

##### What it does
First vocab term:

Topology - The services and apps you deploy and their placement in the cluster. Here's an example:
Node - A node in the cluster. Yes, it somewhat forces you to think like that.

> I have a 5 nodes cluster, three Kafka brokers and two Kafka Connect workers.

That's pretty much enough for it to make that happen for you, actually (never mind you also need Zookeeper and possibly
Schema registry for it to work). Here's the topology.txt for that, however:

node_count        = 5

kafka_cfg         = 1,2,3:9092

kafka_connect_cfg = 4,5:8083

##### What's with that format
It's more or less old property files flavor. Key = value. Really not hip.

 * service config definitions always end in _cfg. zookeeper_cfg means you have a 'zookeeper' service in your topology 
 * Then, you say <nodes>:<ports>
 
The nodes part is what node you want that on. 1,2,3 means you want that service on nodes one, two and three.
Duplicates are allowed. You can have 1,1,1 and that means three instances of this service on node one
 
The ports part is a bit trickier. Think of it more like a port range. Here's the simple case:
 
zookeeper_cfg   = 1,2,3:2181,2888,3888
 
This means I want Zookeeper on nodes 1, 2, and 3 and it'll bind ports 2181, 2888, 3888 on **each** of them
 
Here's the more 'complicated' case, lol
 
zookeeper_cfg   = 1,1,1:2181,2888,3888
 
This is valid valid config. And this means I want three zookeepers on node 1, and it'll bind ports **starting with** 
2181, 2888, 3888. In practice you'll end up with the first instance binding to 2181, 2888 and 3888, the second one to
2182,2889 and 3889 and so on. You get the idea. It'll be even 'messier' if your port ranges overlap. It knows to generate
ports to avoid overlapping so if your ranges overlap you won't get them in order. Find consolation in the fact that
it'll actually work.
 
##### Other configuration?

You specify it into service folders in the dir of the topology definition:
```
|
 - topology.txt
 - services
 | - zookeeper
   | - config
   | - swarm-service-~.yml.tmpl
 | - prometheus
   | - config
   | - prometheus.yml.tmpl
```
   
##### What else?

It has modules so that you can grab a module, configure and run it. Boom.

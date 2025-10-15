package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ReceiveAddress holds the schema definition for the ReceiveAddress entity.
type ReceiveAddress struct {
	ent.Schema
}

// Mixin of the ReceiveAddress.
func (ReceiveAddress) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

// Fields of the ReceiveAddress.
func (ReceiveAddress) Fields() []ent.Field {
	return []ent.Field{
		field.String("address"), // Removed .Unique() to allow address reuse across multiple orders
		field.Bytes("salt").Optional(),
		
		// Status - updated with pool management values
		field.Enum("status").
			Values(
				"pool_ready",      // Deployed and available in pool
				"pool_assigned",   // Assigned to an order (in use)
				"pool_processing", // Order is being processed
				"pool_completed",  // Order completed, ready for recycling
				"unused",          // Legacy: Not deployed
				"used",            // Legacy: Was used for an order
				"expired",         // Legacy: Expired
			).
			Default("unused"),
		
		// Deployment tracking
		field.Bool("is_deployed").
			Default(false).
			Comment("Whether the smart account is deployed on-chain"),
		field.Int64("deployment_block").
			Optional().
			Comment("Block number where account was deployed"),
		field.String("deployment_tx_hash").
			MaxLen(70).
			Optional().
			Comment("Transaction hash of deployment"),
		field.Time("deployed_at").
			Optional().
			Comment("Timestamp when deployed"),
		
		// Network identification
		field.String("network_identifier").
			Optional().
			Comment("Network identifier (e.g., base-sepolia)"),
		field.Int64("chain_id").
			Optional().
			Comment("Chain ID (e.g., 84532)"),
		
		// Pool management
		field.Time("assigned_at").
			Optional().
			Comment("When address was assigned to an order"),
		field.Time("recycled_at").
			Optional().
			Comment("When address was returned to pool"),
		field.Int("times_used").
			Default(0).
			Comment("Number of times address has been reused"),
		
		// Existing fields
		field.Int64("last_indexed_block").Optional(),
		field.Time("last_used").Optional(),
		field.String("tx_hash").
			MaxLen(70).
			Optional(),
		field.Time("valid_until").Optional(),
	}
}

// Edges of the ReceiveAddress.
func (ReceiveAddress) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("payment_order", PaymentOrder.Type).
			Ref("receive_address").
			Unique(),
	}
}

// Indexes of the ReceiveAddress for efficient pool queries.
func (ReceiveAddress) Indexes() []ent.Index {
	return []ent.Index{
		// Fast lookup for available addresses in pool
		index.Fields("status", "is_deployed", "network_identifier"),
		
		// Fast lookup by chain
		index.Fields("chain_id", "status"),
		
		// Track reuse count for pool maintenance
		index.Fields("times_used"),
	}
}
